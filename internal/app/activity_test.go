package app

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/activity"
	"podmutt/internal/discovery"
)

type blockedDirectory struct {
	entered chan struct{}
	release chan struct{}
}

func (d *blockedDirectory) Top(ctx context.Context) ([]discovery.Show, error) {
	return d.Search(ctx, "")
}
func (d *blockedDirectory) Search(ctx context.Context, _ string) ([]discovery.Show, error) {
	close(d.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-d.release:
		return []discovery.Show{{Title: "Result", FeedURL: "https://example.test/rss"}}, nil
	}
}

func TestDiscoverWorkPublishesSharedActivityAndCancellation(t *testing.T) {
	librarySession(t, func(s *session) {
		tracker := activity.FromContext(s.ctx)
		directory := &blockedDirectory{make(chan struct{}), make(chan struct{})}
		m := browserLibrary(t, s, directory)
		cmd := m.searchDirectory("science")
		result := make(chan tea.Msg, 1)
		go func() { result <- cmd() }()
		select {
		case <-directory.entered:
		case <-time.After(time.Second):
			t.Fatal("search did not start")
		}
		snapshot := tracker.Snapshot()
		if len(snapshot.Operations) != 1 || snapshot.Operations[0].Label != "Searching Apple Podcasts…" {
			t.Fatal(snapshot)
		}
		signal := m.waitActivity()
		msg := signal()
		_, tick := m.shell.Update(msg)
		if tick == nil {
			t.Fatal("no activity subscription/timer")
		}
		// Cancel without allowing the command to return a successful result.
		m.cancelDirectory()
		select {
		case msg := <-result:
			loaded := msg.(directoryLoaded)
			if !errors.Is(loaded.err, context.Canceled) {
				t.Fatal(loaded.err)
			}
			m.applyDirectory(loaded)
		case <-time.After(time.Second):
			t.Fatal("search survived cancellation")
		}
		if len(tracker.Snapshot().Operations) != 0 || len(m.browse.shows) != 0 {
			t.Fatal("canceled work stuck or stale reply accepted")
		}
	})
}

func TestTrackedWorkNestedFailurePanicAndSubscriptionCancellation(t *testing.T) {
	tracker := activity.New()
	ctx, cancel := context.WithCancel(activity.WithTracker(context.Background(), tracker))
	s := shell{s: &session{ctx: ctx}}
	cmd := work(ctx, "Outer", func(ctx context.Context) tea.Msg {
		inner := work(ctx, "Inner", func(context.Context) tea.Msg {
			if len(tracker.Snapshot().Operations) != 1 {
				t.Fatal("nested double count")
			}
			return actionError{errors.New("failed")}
		})
		return inner()
	})
	if _, ok := cmd().(actionError); !ok || len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("failed work not cleaned up")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic swallowed")
			}
		}()
		work(ctx, "Panic", func(context.Context) tea.Msg { panic("test") })()
	}()
	if len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("panic leaked activity")
	}
	// Drain queued updates before waiting for shutdown.
	select {
	case <-tracker.Changed():
	default:
	}
	wait := s.waitActivity()
	cancel()
	if wait() != nil {
		t.Fatal("activity subscription survived shutdown")
	}
	if (shell{s: &session{}}).waitActivity() != nil {
		t.Fatal("tracker required by test-only sessions")
	}
	for command, want := range map[string]string{"theme:colors": "Changing theme…", "country:cycle": "Changing discovery country…", "pause": "Pausing playback…", "play": "Starting playback…", "stop": "Stopping playback…", "next": "Starting next episode…", "seek-forward": "Seeking…", "toggle": "Updating playback…"} {
		if commandLabel(command) != want {
			t.Fatal(command)
		}
	}
}
