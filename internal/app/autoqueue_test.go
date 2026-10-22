package app

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/playback"
	"podmutt/internal/store"
)

func localAutoFeed(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "audio") {
			t.Error("autoqueue fetched audio")
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
func syncStateForAuto(t *testing.T, s *session) playback.State {
	t.Helper()
	state, err := s.playback.Snapshot(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestCachedStartupAutoqueuesOnceAndRemainsStopped(t *testing.T) {
	librarySession(t, func(s *session) {
		server := localAutoFeed(t)
		id, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: server, Subscribed: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.UpsertEpisodes(s.ctx, id, []store.Episode{{GUID: "latest", AudioURL: server + "/audio"}}); err != nil {
			t.Fatal(err)
		}
		if err := runLibrary(s, strings.NewReader("q"), io.Discard); err != nil {
			t.Fatal(err)
		}
		state := syncStateForAuto(t, s)
		if state.Status != "stopped" || state.Current != nil || len(state.Queue) != 1 {
			t.Fatal("cached startup/autoplay", state)
		}
		if err := s.playback.SetQueue(s.ctx, nil); err != nil {
			t.Fatal(err)
		}
		if err := runLibrary(s, strings.NewReader("q"), io.Discard); err != nil {
			t.Fatal(err)
		}
		if len(syncStateForAuto(t, s).Queue) != 0 {
			t.Fatal("startup undid removal")
		}
	})
}

func TestRefreshAndConditionalRefreshReconcileLatest(t *testing.T) {
	librarySession(t, func(s *session) {
		a, _ := seededLibrary(t, s)
		m, err := newLibrary(s, feed.New(s.store, nil, time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan feed.Result)
		close(results)
		m.results = results
		var drain func(tea.Cmd)
		calls := 0
		drain = func(cmd tea.Cmd) {
			if cmd == nil {
				return
			}
			calls++
			if calls > 100 {
				t.Fatal("unbounded command chain")
			}
			msg := cmd()
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, part := range batch {
					drain(part)
				}
				return
			}
			drain(updateLibrary(&m, msg))
		}
		drain(updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: a, NotModified: true}, ok: true}))
		state := syncStateForAuto(t, s)
		if len(state.Queue) != 2 || state.Queue[0].Title != "Alpha" || state.Queue[1].Title != "Zulu" {
			t.Fatal("304 did not reconcile latest", state)
		}
		if err := s.playback.SetQueue(s.ctx, nil); err != nil {
			t.Fatal(err)
		}
		drain(updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: a}, ok: true}))
		if len(syncStateForAuto(t, s).Queue) != 0 {
			t.Fatal("unchanged refresh resurrected")
		}
		updateLibrary(&m, latestQueued{err: errors.New("queue failure")})
		if !strings.Contains(m.backgroundError.Summary, "Update latest episodes failed") {
			t.Fatal("queue error hidden")
		}
		if msg := subscriptionEdited(s.ctx, s.playback, a, true, errors.New("subscribe failure")); msg.err == nil {
			t.Fatal("subscription failure hidden")
		}
	})
}
