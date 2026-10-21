package playback

import (
	"context"
	"testing"
	"time"

	"podmutt/internal/activity"
)

func TestExternalControlUsesSessionActivityWithoutTrackingIdleSnapshots(t *testing.T) {
	db, p, episodes := setup(t)
	tracker := activity.New()
	ctx := activity.WithTracker(context.Background(), tracker)
	entered, release := make(chan struct{}), make(chan struct{})
	e, err := newEngine(ctx, db, func(context.Context) (Player, error) { close(entered); <-release; return p, nil }, make(chan time.Time), func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close(context.Background())
	if _, err := e.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tracker.Snapshot().Revision != 0 {
		t.Fatal("idle snapshot tracked")
	}
	done := make(chan error, 1)
	go func() { done <- e.PlayEpisode(context.Background(), episodes[0].ID) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("player did not start")
	}
	s := tracker.Snapshot()
	if len(s.Operations) != 1 || s.Operations[0].Label != "Updating playback…" {
		close(release)
		t.Fatal(s)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("control stuck")
	}
	if len(tracker.Snapshot().Operations) != 0 {
		t.Fatal("control left activity running")
	}
}
