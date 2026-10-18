package store

import (
	"math"
	"testing"
)

func TestActivateStackAtomicity(t *testing.T) {
	db, _ := openStore(t)
	feed, err := db.UpsertFeed(ctx, Feed{URL: "https://example.test/feed", Title: "A show"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertEpisodes(ctx, feed, []Episode{{GUID: "a", AudioURL: "https://example.test/a"}, {GUID: "b", AudioURL: "https://example.test/b"}}); err != nil {
		t.Fatal(err)
	}
	episodes, err := db.Episodes(ctx, feed)
	if err != nil {
		t.Fatal(err)
	}
	a, b := episodes[0].ID, episodes[1].ID
	if err := db.Checkpoint(ctx, a, 12); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, b, true); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceQueue(ctx, []int64{b}); err != nil {
		t.Fatal(err)
	}
	if err := db.Activate(ctx, b, 0, []int64{a, 999999}); err == nil {
		t.Fatal("invalid queue accepted")
	}
	current, err := db.Current(ctx)
	if err != nil || current.EpisodeID != a || current.PositionSeconds != 12 {
		t.Fatal("current rollback", current, err)
	}
	history, err := db.Playback(ctx, b)
	if err != nil || !history.Completed {
		t.Fatal("completion rollback", history, err)
	}
	pending, err := db.Queue(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != b {
		t.Fatal("queue rollback", pending, err)
	}
	if err := db.Activate(ctx, b, 0, []int64{a}); err != nil {
		t.Fatal(err)
	}
	current, err = db.Current(ctx)
	if err != nil || current.EpisodeID != b || current.Completed {
		t.Fatal("activation", current, err)
	}
	pending, err = db.Queue(ctx)
	if err != nil || len(pending) != 1 || pending[0].ID != a || pending[0].PositionSeconds != 12 || pending[0].FeedTitle != "A show" {
		t.Fatal("remaining stack", pending, err)
	}
	for _, position := range []float64{-1, math.NaN(), math.Inf(1)} {
		if err := db.Activate(ctx, a, position, nil); err == nil {
			t.Fatal("invalid position accepted")
		}
	}
	if err := db.Activate(ctx, 999999, 0, nil); err == nil {
		t.Fatal("unknown current accepted")
	}
}
