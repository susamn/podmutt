package app

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"podmutt/internal/profile"
	"podmutt/internal/store"
)

// Run by the same required null-audio job as internal/player. Unlike direct
// Player.Close tests this crosses the real session -> actor -> exec context
// lifetime, catching premature process kills on ordinary foreground quit.
func TestRealMPVSessionQuitCheckpointsBeforeProcessCancellation(t *testing.T) {
	path := os.Getenv("PODMUTT_TEST_MPV")
	if path == "" {
		t.Skip("run make mpv-test")
	}
	real, err := exec.LookPath(path)
	if err != nil {
		t.Fatal(err)
	}
	wav := make([]byte, 44+5*8000*2)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 8000)
	binary.LittleEndian.PutUint32(wav[28:], 16000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(wav)-44))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(wav)
	}))
	defer server.Close()
	wrapper := filepath.Join(t.TempDir(), "mpv-null")
	if err = os.WriteFile(wrapper, []byte("#!/bin/sh\nexec \"$PODMUTT_REAL_MPV\" --ao=null \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PODMUTT_REAL_MPV", real)
	paths := isolatedProfile(t)
	if err = paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	cfg := profile.Defaults()
	cfg.MPVPath = wrapper
	err = withSession(cfg, paths, func(s *session) error {
		id, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: server.URL + "/rss", Subscribed: true})
		if err != nil {
			return err
		}
		if err = s.store.UpsertEpisodes(s.ctx, id, []store.Episode{{GUID: "one", AudioURL: server.URL + "/silence.wav"}}); err != nil {
			return err
		}
		episodes, err := s.store.Episodes(s.ctx, id)
		if err != nil {
			return err
		}
		if err = s.playback.PlayEpisode(s.ctx, episodes[0].ID); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			state, err := s.playback.Snapshot(s.ctx)
			if err != nil {
				return err
			}
			if state.Current != nil && state.Current.PositionSeconds > 0.02 {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("real mpv never advanced: %+v", state)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
	if err != nil {
		t.Fatal("ordinary quit killed player or lost final checkpoint:", err)
	}
	db, err := store.Open(t.Context(), filepath.Join(paths.State, "podcasts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	current, err := db.Current(t.Context())
	if err != nil || current.PositionSeconds <= 0.02 {
		t.Fatal("missing final checkpoint", current, err)
	}
	if _, err = os.Lstat(filepath.Join(paths.Runtime, "control.sock")); !os.IsNotExist(err) {
		t.Fatal("control socket left behind", err)
	}
}
