package macmedia

import (
	"context"
	"errors"
	"testing"
	"time"

	"podmutt/internal/playback"
)

func awaitCleanup(t *testing.T, s *Service, d *fakeDriver) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("worker leaked")
	}
	if d.closed.Load() != 1 || d.callback("play") {
		t.Fatal("handler not automatically removed")
	}
	s.Close()
	if d.closed.Load() != 1 {
		t.Fatal("double removal")
	}
}
func TestNativeDriverFailureAndSourceLossAutomaticallyUnregister(t *testing.T) {
	t.Run("framework metadata failure", func(t *testing.T) {
		b, d := fixtures()
		sentinel := errors.New("framework unavailable")
		d.updateErr = sentinel
		s, err := start(context.Background(), b, d)
		if err != nil {
			t.Fatal(err)
		}
		b.updates <- playback.State{}
		awaitCleanup(t, s, d)
		if !errors.Is(s.Err(), sentinel) {
			t.Fatal(s.Err())
		}
	})
	t.Run("actor watcher closed", func(t *testing.T) {
		b, d := fixtures()
		s, err := start(context.Background(), b, d)
		if err != nil {
			t.Fatal(err)
		}
		close(b.updates)
		awaitCleanup(t, s, d)
	})
	t.Run("parent cancelled without Close", func(t *testing.T) {
		b, d := fixtures()
		ctx, cancel := context.WithCancel(context.Background())
		s, err := start(ctx, b, d)
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		awaitCleanup(t, s, d)
	})
	t.Run("cancel during subscription never installs", func(t *testing.T) {
		b, d := fixtures()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		b.watchCancel = cancel
		if _, err := start(ctx, b, d); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if d.callback != nil {
			t.Fatal("installed after watcher cancellation")
		}
	})
	t.Run("cancelled startup never installs", func(t *testing.T) {
		b, d := fixtures()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := start(ctx, b, d); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if d.callback != nil || d.closed.Load() != 0 {
			t.Fatal("touched native driver after cancellation")
		}
	})
}
