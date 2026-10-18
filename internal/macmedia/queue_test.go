package macmedia

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"podmutt/internal/playback"
	"podmutt/internal/store"
)

// Real actor Watch -> service -> driver, not synthetic metadata or Snapshot
// injection: queue-only changes must refresh native capabilities while stopped.
func TestLiveQueueAvailability(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "podcasts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	feed, err := db.UpsertFeed(ctx, store.Feed{URL: "https://example.test/feed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertEpisodes(ctx, feed, []store.Episode{{GUID: "a", Title: "Queued", AudioURL: "https://example.test/a"}}); err != nil {
		t.Fatal(err)
	}
	episodes, err := db.Episodes(ctx, feed)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := playback.New(ctx, db, func(context.Context) (playback.Player, error) {
		t.Error("enqueue started player")
		return nil, context.Canceled
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close(ctx)
	_, d := fixtures()
	s, err := start(ctx, engine, d)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	expect := func(next bool) {
		t.Helper()
		select {
		case m := <-d.updates:
			if m.CanNext != next || m.Playing {
				t.Fatalf("capability: %+v", m)
			}
		case <-time.After(time.Second):
			t.Fatal("queue publication stalled")
		}
	}
	expect(false)
	for range 3 {
		if err := engine.Enqueue(ctx, episodes[0].ID); err != nil {
			t.Fatal(err)
		}
		expect(true)
		if err := engine.SetQueue(ctx, nil); err != nil {
			t.Fatal(err)
		}
		expect(false)
	}
}

func TestQueueAvailabilityAcrossTransportStates(t *testing.T) {
	for _, status := range []playback.Status{playback.Stopped, playback.Playing, playback.Paused, playback.Failed} {
		for _, queued := range []bool{false, true} {
			state := playback.State{Status: status}
			if queued {
				state.Queue = []store.Episode{{ID: 2}}
			}
			if got := info(state).CanNext; got != queued {
				t.Fatalf("%s queue=%v: next=%v", status, queued, got)
			}
		}
	}
}
