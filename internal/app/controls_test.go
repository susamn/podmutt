package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/control"
	"podmutt/internal/playback"
	"podmutt/internal/profile"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestForegroundServicesAndKeyboardShareActions(t *testing.T) {
	isolatedProfile(t)
	err := withProfile(func(cfg profile.Config, paths profile.Paths) error {
		return withSession(cfg, paths, func(s *session) error {
			var output bytes.Buffer
			if err := Command("status", &output); err != nil || !strings.Contains(output.String(), `"status":"stopped"`) {
				t.Fatalf("status: %s, %v", output.String(), err)
			}
			if err := Command("stop", &output); err != nil {
				t.Fatal(err)
			}
			m := newShell(s)
			updated, cmd := m.Update(m.Init()())
			m = updated.(shell)
			if cmd == nil {
				t.Fatal("not subscribed to player updates")
			}
			updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
			m = updated.(shell)
			for _, key := range []rune{'s', 'n'} {
				updated, cmd = m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
				m = updated.(shell)
				if cmd == nil || cmd() != nil {
					t.Fatal("keyboard action failed")
				}
			}
			_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeySpace})
			failure, ok := cmd().(actionError)
			if !ok || !errors.Is(failure.error, playback.ErrNoEpisode) {
				t.Fatalf("toggle: %+v", failure)
			}
			updated, _ = m.Update(failure)
			m = updated.(shell)
			if !strings.Contains(m.View().Content, "no current episode") {
				t.Fatal("action error hidden")
			}
			updated, cmd = m.Update(playbackUpdate{state: playback.State{Status: playback.Failed, Current: &store.Episode{Title: "Title"}, Error: "offline"}, ok: true})
			m = updated.(shell)
			if cmd == nil || !strings.Contains(m.View().Content, "offline") {
				t.Fatal("playback state not visible")
			}
			if _, cmd := m.Update(playbackUpdate{}); cmd != nil {
				t.Fatal("closed updates resubscribed")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Command("status", new(bytes.Buffer)); !errors.Is(err, control.ErrNotRunning) {
		t.Fatalf("endpoint survived: %v", err)
	}
}

func TestPlaybackRenderingPreservesQueueProgressAndMediaWarnings(t *testing.T) {
	m := newShell(&session{mediaError: errors.New("session bus unavailable")})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 32})
	m = updated.(shell)
	state := playback.State{
		Status: playback.Failed, Error: "stream failed",
		Current: &store.Episode{ID: 1, Title: "Current episode", PositionSeconds: 42, DurationSeconds: 120},
		Queue:   []store.Episode{{ID: 7, Title: "<b>Queued episode</b>"}},
	}
	updated, cmd := m.Update(playbackUpdate{state: state, ok: true})
	m = updated.(shell)
	if cmd == nil {
		t.Fatal("lost playback subscription")
	}
	queued, ok := m.Model.Selected(ui.Queue)
	if !ok || queued.ID != "1" || len(m.Model.Visible(ui.Queue)) != 2 || !strings.Contains(m.Model.Visible(ui.Queue)[1].Title, "Queued episode") {
		t.Fatalf("queue lost: %+v", queued)
	}
	for _, text := range []string{"0:42 / 2:00", "stream failed", "session bus unavailable"} {
		if !strings.Contains(m.View().Content, text) {
			t.Fatalf("rendering lost %q", text)
		}
	}
	if m.state.Current.ID != 1 {
		t.Fatal("selected playback state lost")
	}
}

func TestClientsAreSideEffectFree(t *testing.T) {
	paths := isolatedProfile(t)
	if err := Command("status", new(bytes.Buffer)); !errors.Is(err, control.ErrNotRunning) {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.State); !os.IsNotExist(err) {
		t.Fatal("client created state")
	}
	t.Setenv("HOME", "")
	if err := Command("status", new(bytes.Buffer)); err == nil {
		t.Fatal("missing home ignored")
	}
}

func TestServiceStartupAndPlayerFailure(t *testing.T) {
	paths := isolatedProfile(t)
	if err := paths.Prepare(); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("callback failure")
	if err := withSession(profile.Defaults(), paths, func(s *session) error {
		id, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: "https://example.test/feed", Subscribed: true})
		if err != nil {
			return err
		}
		if err := s.store.UpsertEpisodes(s.ctx, id, []store.Episode{{AudioURL: "https://example.test/a.mp3"}}); err != nil {
			return err
		}
		return sentinel
	}); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	cfg := profile.Defaults()
	cfg.MPVPath = filepath.Join(paths.State, "missing-mpv")
	if err := withSession(cfg, paths, func(s *session) error {
		items, err := s.store.Episodes(context.Background(), 0)
		if err != nil {
			return err
		}
		if err := s.playback.PlayEpisode(s.ctx, items[0].ID); err == nil {
			t.Fatal("missing mpv accepted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.Runtime, "control.sock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := withSession(cfg, paths, func(*session) error { t.Fatal("unsafe endpoint accepted"); return nil }); err == nil {
		t.Fatal("startup error ignored")
	}
	os.Remove(filepath.Join(paths.Runtime, "control.sock"))
	if err := os.RemoveAll(filepath.Join(paths.State, "podcasts.db")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(paths.State, "podcasts.db"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := withSession(cfg, paths, func(*session) error { t.Fatal("bad db accepted"); return nil }); err == nil {
		t.Fatal("db error ignored")
	}
}

func TestUIActionGuards(t *testing.T) {
	calls := 0
	model := ui.New(ui.Data{}).WithActions(func(string) tea.Cmd { calls++; return nil })
	updated, _ := model.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	model = updated.(ui.Model)
	model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if calls != 0 {
		t.Fatal("filter text dispatched playback")
	}
}
