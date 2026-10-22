package store

import (
	"database/sql"
	"errors"
	"testing"
)

func TestClearCurrentPreservesHistoryQueueAndRestartState(t *testing.T) {
	db, path := openStore(t)
	feed := addFeed(t, db, "clear-current", true)
	episodes := addEpisodes(t, db, feed)
	a, b := episodes[0].ID, episodes[1].ID
	if err := db.Checkpoint(ctx, a, 52.5); err != nil {
		t.Fatal(err)
	}
	if err := db.SetCompleted(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceQueue(ctx, []int64{b}); err != nil {
		t.Fatal(err)
	}
	if err := db.ClearCurrent(ctx, b); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("wrong identity cleared current", err)
	}
	if got, err := db.Current(ctx); err != nil || got.EpisodeID != a {
		t.Fatal(got, err)
	}
	if err := db.ClearCurrent(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := db.ClearCurrent(ctx, a); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("stale removal accepted", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.Current(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	if got, err := reopened.Playback(ctx, a); err != nil || got.PositionSeconds != 52.5 || !got.Completed {
		t.Fatal("history lost", got, err)
	}
	if queue, err := reopened.Queue(ctx); err != nil || len(queue) != 1 || queue[0].ID != b {
		t.Fatal("pending changed", queue, err)
	}
	if err := db.ClearCurrent(ctx, a); err == nil {
		t.Fatal("closed DB accepted removal")
	}
}
