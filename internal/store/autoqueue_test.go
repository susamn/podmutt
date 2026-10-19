package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestReceiptsPreserveManualQueueAndNeverResurrect(t *testing.T) {
	s, _ := openStore(t)
	add := func(url string, sub bool, episodes []Episode) int64 {
		t.Helper()
		id, err := s.UpsertFeed(ctx, Feed{URL: url, Subscribed: sub})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.UpsertEpisodes(ctx, id, episodes); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := add("https://example.test/a", true, []Episode{{GUID: "z", AudioURL: "https://example.test/old", PublishedAt: time.Unix(100, 0)}, {GUID: "b", AudioURL: "https://example.test/b", PublishedAt: time.Unix(200, 0)}, {GUID: "a", AudioURL: "https://example.test/a", PublishedAt: time.Unix(200, 0)}})
	b := add("https://example.test/b", false, []Episode{{GUID: "b", AudioURL: "https://example.test/manual"}})
	c := add("https://example.test/c", true, []Episode{{GUID: "c", AudioURL: "https://example.test/current"}})
	byGUID := func(feed int64, guid string) int64 {
		t.Helper()
		episodes, err := s.Episodes(ctx, feed)
		if err != nil {
			t.Fatal(err)
		}
		for _, episode := range episodes {
			if episode.GUID == guid {
				return episode.ID
			}
		}
		t.Fatal("missing episode")
		return 0
	}
	manual, current, latest := byGUID(b, "b"), byGUID(c, "c"), byGUID(a, "a")
	if err := s.ReplaceQueue(ctx, []int64{manual}); err != nil {
		t.Fatal(err)
	}
	queue, err := s.EnqueueLatest(ctx, current, 0)
	if err != nil || len(queue) != 3 || queue[0].ID != manual || queue[1].ID != latest || queue[2].ID != byGUID(a, "b") {
		t.Fatal("latest/tie/manual ordering", queue, err)
	}
	if err := s.ReplaceQueue(ctx, []int64{manual}); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, 0)
	if err != nil || len(queue) != 1 || queue[0].ID != manual {
		t.Fatal("removed/current receipt resurrected", queue, err)
	}
	if err := s.UpsertEpisodes(ctx, a, []Episode{{GUID: "new", AudioURL: "https://example.test/new", PublishedAt: time.Unix(300, 0)}}); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, a)
	if err != nil || len(queue) != 2 || queue[1].GUID != "new" {
		t.Fatal("new head missing", queue, err)
	}
	// Unsubscribe keeps queue/history/receipts; resubscribe cannot undo removal.
	if err := s.SetSubscribed(ctx, a, false); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceQueue(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSubscribed(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, a)
	if err != nil || len(queue) != 0 {
		t.Fatal("resubscribe resurrected", queue, err)
	}
	if err := s.SetSubscribed(ctx, b, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCompleted(ctx, manual, true); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, b)
	if err != nil || len(queue) != 0 {
		t.Fatal("completed latest queued", queue, err)
	}
	if err := s.SetCompleted(ctx, manual, false); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, b)
	if err != nil || len(queue) != 0 {
		t.Fatal("receipt missing for completed latest", queue, err)
	}
	if _, err := s.EnqueueLatest(ctx, 0, -1); err == nil {
		t.Fatal("invalid feed accepted")
	}
}

func TestLatestReceiptQueueAtomicRollback(t *testing.T) {
	s, _ := openStore(t)
	id, err := s.UpsertFeed(ctx, Feed{URL: "https://example.test/feed", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertEpisodes(ctx, id, []Episode{{GUID: "a", AudioURL: "https://example.test/a"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER reject_auto BEFORE INSERT ON queue BEGIN SELECT RAISE(ABORT,'injected queue failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnqueueLatest(ctx, 0, 0); err == nil {
		t.Fatal("queue error ignored")
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM autoqueue_seen`).Scan(&count); err != nil || count != 0 {
		t.Fatal("receipt committed on failure", count, err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_auto`); err != nil {
		t.Fatal(err)
	}
	queue, err := s.EnqueueLatest(ctx, 0, 0)
	if err != nil || len(queue) != 1 {
		t.Fatal("retry failed", queue, err)
	}
	// Already queued is consumed too; it is not inserted again after removal.
	if _, err := s.db.Exec(`DELETE FROM autoqueue_seen`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnqueueLatest(ctx, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceQueue(ctx, nil); err != nil {
		t.Fatal(err)
	}
	queue, err = s.EnqueueLatest(ctx, 0, 0)
	if err != nil || len(queue) != 0 {
		t.Fatal("queued receipt missing", queue, err)
	}
	s.Close()
	if _, err := s.EnqueueLatest(ctx, 0, 0); err == nil {
		t.Fatal("closed database ignored")
	}
}

func TestUpgradeV1PreservesListeningState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "podcasts.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, []string{schema}); err != nil {
		t.Fatal(err)
	}
	old := &Store{db: db}
	id, err := old.UpsertFeed(ctx, Feed{URL: "https://example.test/feed", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := old.UpsertEpisodes(ctx, id, []Episode{{GUID: "a", AudioURL: "https://example.test/a"}}); err != nil {
		t.Fatal(err)
	}
	episodes, err := old.Episodes(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Checkpoint(ctx, episodes[0].ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := old.ReplaceQueue(ctx, []int64{episodes[0].ID}); err != nil {
		t.Fatal(err)
	}
	old.Close()
	upgraded, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	current, err := upgraded.Current(ctx)
	if err != nil || current.PositionSeconds != 42 {
		t.Fatal("upgrade lost progress", current, err)
	}
	queue, err := upgraded.EnqueueLatest(ctx, current.EpisodeID, 0)
	if err != nil || len(queue) != 1 || queue[0].ID != current.EpisodeID {
		t.Fatal("upgrade lost pending", queue, err)
	}
	if err := upgraded.ReplaceQueue(ctx, nil); err != nil {
		t.Fatal(err)
	}
	upgraded.Close()
	again, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	queue, err = again.EnqueueLatest(ctx, 0, 0)
	if err != nil || len(queue) != 0 {
		t.Fatal("receipt lost on reopen", queue, err)
	}
}
