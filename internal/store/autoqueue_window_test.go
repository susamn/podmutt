package store

import (
	"fmt"
	"testing"
	"time"
)

func releaseWindow(t *testing.T, s *Store) []Episode {
	t.Helper()
	id, err := s.UpsertFeed(ctx, Feed{URL: "https://window.test/rss", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertEpisodes(ctx, id, []Episode{
		{GUID: "old", AudioURL: "https://window.test/old", PublishedAt: time.Unix(100, 0)},
		{GUID: "second", AudioURL: "https://window.test/second", PublishedAt: time.Unix(200, 0)},
		{GUID: "latest", AudioURL: "https://window.test/latest", PublishedAt: time.Unix(300, 0)},
	}); err != nil {
		t.Fatal(err)
	}
	episodes, err := s.Episodes(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return episodes
}

func TestTwoReleaseWindowDoesNotBackfillOrResurrect(t *testing.T) {
	s, _ := openStore(t)
	rows := releaseWindow(t, s)
	if err := s.SetCompleted(ctx, rows[1].ID, true); err != nil {
		t.Fatal(err)
	}
	queue, err := s.EnqueueLatest(ctx, rows[0].ID, 0)
	if err != nil || len(queue) != 0 {
		t.Fatal("current/completed releases caused older backfill", queue, err)
	}
	if err := s.SetCompleted(ctx, rows[1].ID, false); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, 0)
	if err != nil || len(queue) != 0 {
		t.Fatal("window receipts did not survive current/completion changes", queue, err)
	}
	// New release admits only the new head; the shifted second was already seen.
	if err := s.UpsertEpisodes(ctx, rows[0].FeedID, []Episode{{GUID: "new", AudioURL: "https://window.test/new", PublishedAt: time.Unix(400, 0)}}); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, rows[0].FeedID)
	if err != nil || len(queue) != 1 || queue[0].GUID != "new" {
		t.Fatal("arrival replayed the old window", queue, err)
	}
}

func TestTwoReleaseWindowRollsBackTogetherAndPreservesManualWork(t *testing.T) {
	s, _ := openStore(t)
	rows := releaseWindow(t, s)
	if err := s.ReplaceQueue(ctx, []int64{rows[2].ID}); err != nil {
		t.Fatal(err)
	}
	trigger := fmt.Sprintf(`CREATE TRIGGER reject_second BEFORE INSERT ON queue WHEN NEW.episode_id=%d BEGIN SELECT RAISE(ABORT,'second release failed'); END`, rows[1].ID)
	if _, err := s.db.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnqueueLatest(ctx, 0, 0); err == nil {
		t.Fatal("second write failure ignored")
	}
	var seen int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM autoqueue_seen`).Scan(&seen); err != nil || seen != 0 {
		t.Fatal("partial window receipt committed", seen, err)
	}
	if queue, err := s.Queue(ctx); err != nil || len(queue) != 1 || queue[0].ID != rows[2].ID {
		t.Fatal("failed reconcile changed manual work", queue, err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_second`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		queue, err := s.EnqueueLatest(ctx, 0, 0)
		if err != nil || len(queue) != 3 || queue[0].ID != rows[2].ID || queue[1].ID != rows[0].ID || queue[2].ID != rows[1].ID {
			t.Fatal("retry/order/duplicate invariant", queue, err)
		}
	}
}
