package app

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/chapters"
	"podmutt/internal/feed"
	"podmutt/internal/playback"
	"podmutt/internal/store"
)

func TestUpdateFlowStalePlaybackRejection(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		// Initial authoritative state at revision 10
		app.state.Revision = 10
		app.shell.state.Revision = 10

		// Send an older playback update with revision 5
		staleState := playback.State{
			Revision: 5,
			Status:   playback.Playing,
		}
		next, cmd := app.Update(playbackUpdate{state: staleState, ok: true})
		updated := next.(libraryShell)

		// State should NOT be updated to the stale revision
		if updated.state.Revision != 10 {
			t.Fatalf("expected revision 10 to be retained, got %d", updated.state.Revision)
		}
		if cmd == nil {
			t.Fatal("expected wait command from stale update rejection")
		}
	})
}

func TestUpdateFlowPostUpdateClosesChaptersOnPlaybackChange(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		// Set active chapter request for episode 100
		app.chapterRequest = chapterRequest{episodeID: 100}
		app.chapterList = []chapters.Chapter{} // non-nil indicator

		// State currently playing episode 200
		app.state.Current = &store.Episode{ID: 200}

		// Run postUpdate directly
		next, _ := app.postUpdate(nil)
		updated := next.(libraryShell)

		// Chapters must be closed because episode ID changed
		if updated.chapterList != nil {
			t.Fatal("expected chapters to be closed when chapterRequest episode != playing episode")
		}
	})
}

func TestUpdateFlowQueueFocusAppliedInPostUpdate(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		// Request queue select ID
		app.queueSelectID = "42"
		app.state.Queue = []store.Episode{{ID: 42, Title: "Test Episode"}}
		app.shell = app.shell.withPlayback(app.state)

		next, _ := app.postUpdate(nil)
		updated := next.(libraryShell)

		if updated.queueSelectID != "" {
			t.Fatalf("expected queueSelectID to be consumed, got %q", updated.queueSelectID)
		}
	})
}

func TestUpdateFlowTimerAndAnimationBatched(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		updateLibrary(&app, tea.WindowSizeMsg{Width: 100, Height: 30})

		// Enable progress and animation timers
		app.state.Status = playback.Playing
		app.state.Current = &store.Episode{ID: 1, DurationSeconds: 100, PositionSeconds: 10}
		app.state.ClockAt = time.Now()
		app.shell = app.shell.withPlayback(app.state)

		_, cmd := app.postUpdate(nil)
		if cmd == nil {
			t.Fatal("expected batched command from postUpdate for active playback timers")
		}
	})
}

func TestUpdateFlowDelegatedPathPreservesContext(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		// Send an unrecognized custom message; should flow to delegate
		type unhandledCustomMsg struct{}
		next, _ := app.Update(unhandledCustomMsg{})
		if next == nil {
			t.Fatal("expected valid model from delegate")
		}
	})
}

func TestUpdateFlowEarlyReturnSyncsPostUpdate(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		// Send a settings error message (an early return in updateSettings)
		app.chapterRequest = chapterRequest{episodeID: 999}
		app.chapterList = []chapters.Chapter{}
		app.state.Current = &store.Episode{ID: 1} // different ID

		next, _ := app.Update(settingsError{setting: "theme", err: context.Canceled})
		updated := next.(libraryShell)

		// Even though updateSettings returned early, defer postUpdate must have executed and closed chapters
		if updated.chapterList != nil {
			t.Fatal("expected deferred postUpdate to close chapters on early return path")
		}
	})
}
