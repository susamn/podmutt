package store

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestListeningStatePersists(t *testing.T) {
	s, path := openStore(t)
	feed := addFeed(t, s, "feed", true)
	episodes := addEpisodes(t, s, feed)
	newest, older := episodes[0].ID, episodes[1].ID
	if _, err := s.Current(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("initial current: %v", err)
	}
	if got, err := s.Playback(ctx, newest); err != nil || got != (Playback{EpisodeID: newest}) {
		t.Fatalf("initial progress: %+v, %v", got, err)
	}
	if err := s.Checkpoint(ctx, newest, 42.25); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCompleted(ctx, newest, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(ctx, newest, 60); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(ctx, older, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCompleted(ctx, older, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCompleted(ctx, older, false); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceQueue(ctx, []int64{older, newest, older}); err != nil {
		t.Fatal(err)
	}
	// A metadata refresh cannot overwrite listening history or queue identity.
	if err := s.UpsertEpisodes(ctx, feed, []Episode{episodes[0]}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got, err := reopened.Playback(ctx, newest); err != nil || got != (Playback{newest, 60, true}) {
		t.Fatalf("saved history: %+v, %v", got, err)
	}
	if got, err := reopened.Current(ctx); err != nil || got != (Playback{older, 3, false}) {
		t.Fatalf("saved current: %+v, %v", got, err)
	}
	queue, err := reopened.Queue(ctx)
	if err != nil || len(queue) != 2 || queue[0].ID != older || queue[1].ID != newest || queue[0].PositionSeconds != 3 || !queue[1].Completed {
		t.Fatalf("saved queue: %+v, %v", queue, err)
	}
}

func TestCheckpointValidationAndRollback(t *testing.T) {
	s, _ := openStore(t)
	feed := addFeed(t, s, "feed", true)
	episodes := addEpisodes(t, s, feed)
	id := episodes[0].ID
	if err := s.Checkpoint(ctx, id, 10); err != nil {
		t.Fatal(err)
	}
	for _, position := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := s.Checkpoint(ctx, id, position); err == nil {
			t.Errorf("accepted position %v", position)
		}
	}
	if err := s.Checkpoint(ctx, 999, 1); err == nil {
		t.Fatal("unknown episode checkpointed")
	}
	if err := s.SetCompleted(ctx, 999, true); err == nil {
		t.Fatal("unknown episode completed")
	}
	if _, err := s.Playback(ctx, 999); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown history: %v", err)
	}
	// Force failure after writing position to prove the checkpoint is atomic.
	if _, err := s.db.Exec(`CREATE TRIGGER reject_current BEFORE INSERT ON current_episode
		BEGIN SELECT RAISE(ABORT, 'test rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(ctx, episodes[1].ID, 20); err == nil {
		t.Fatal("expected current-episode failure")
	}
	if got, err := s.Playback(ctx, episodes[1].ID); err != nil || got.PositionSeconds != 0 {
		t.Fatalf("partial position saved: %+v, %v", got, err)
	}
	if got, err := s.Current(ctx); err != nil || got != (Playback{id, 10, false}) {
		t.Fatalf("current changed after failed checkpoint: %+v, %v", got, err)
	}
}

func queueIDs(t *testing.T, s *Store) []int64 {
	t.Helper()
	queue, err := s.Queue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, episode := range queue {
		ids = append(ids, episode.ID)
	}
	return ids
}

func TestQueueReplaceReorderAndRollback(t *testing.T) {
	s, _ := openStore(t)
	feed := addFeed(t, s, "feed", true)
	episodes := addEpisodes(t, s, feed)
	a, b := episodes[0].ID, episodes[1].ID
	for _, ids := range [][]int64{{a, b}, {b, a}, {a}, nil} {
		if err := s.ReplaceQueue(ctx, ids); err != nil {
			t.Fatal(err)
		}
		if got := queueIDs(t, s); !reflect.DeepEqual(got, ids) {
			t.Fatalf("queue = %v, want %v", got, ids)
		}
	}
	if err := s.ReplaceQueue(ctx, []int64{a, a, b, b}); err != nil {
		t.Fatal(err)
	}
	for _, ids := range [][]int64{{b, 999}, {-1}} {
		if err := s.ReplaceQueue(ctx, ids); err == nil {
			t.Fatal("invalid queue accepted")
		}
		if got := queueIDs(t, s); !reflect.DeepEqual(got, []int64{a, b}) {
			t.Fatalf("failed replacement changed queue: %v", got)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.ReplaceQueue(cancelled, nil); err == nil {
		t.Fatal("cancelled queue replacement succeeded")
	}
	if err := s.SetCompleted(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Current(ctx); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("mark-completed changed current episode: %v", err)
	}
}

func TestListeningClosedDatabase(t *testing.T) {
	s, _ := openStore(t)
	s.Close()
	if err := s.Checkpoint(ctx, 1, 0); err == nil {
		t.Fatal("checkpoint succeeded")
	}
	if err := s.SetCompleted(ctx, 1, true); err == nil {
		t.Fatal("completion succeeded")
	}
	if _, err := s.Playback(ctx, 1); err == nil {
		t.Fatal("history query succeeded")
	}
	if _, err := s.Current(ctx); err == nil {
		t.Fatal("current query succeeded")
	}
	if err := s.ReplaceQueue(ctx, nil); err == nil {
		t.Fatal("queue replacement succeeded")
	}
	if _, err := s.Queue(ctx); err == nil {
		t.Fatal("queue query succeeded")
	}
}
