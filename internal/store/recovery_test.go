package store

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCrashStoreChild(t *testing.T) {
	path := os.Getenv("PODMUTT_CRASH_DATABASE")
	if path == "" {
		return
	}
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertFeed(ctx, Feed{URL: "https://offline.test/rss", Title: "Recovery", Subscribed: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpsertEpisodes(ctx, id, []Episode{{GUID: "one", AudioURL: "https://offline.test/one"}, {GUID: "two", AudioURL: "https://offline.test/two"}}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Episodes(ctx, id)
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	if err = s.Checkpoint(ctx, rows[0].ID, 42.5); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCompleted(ctx, rows[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if err = s.ReplaceQueue(ctx, []int64{rows[1].ID, rows[0].ID}); err != nil {
		t.Fatal(err)
	}
	fmt.Println("committed")
	time.Sleep(time.Hour) // parent SIGKILL: no Close/checkpoint cleanup
}
func TestAbruptDeathWALRecoveryAndConsistentBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "podcasts.db")
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	child := exec.CommandContext(deadline, os.Args[0], "-test.run=^TestCrashStoreChild$")
	child.Env = append(os.Environ(), "PODMUTT_CRASH_DATABASE="+path)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != "committed" {
		t.Fatal("crash fixture did not commit")
	}
	if stat, err := os.Stat(path + "-wal"); err != nil || stat.Size() == 0 {
		t.Fatal("fixture did not exercise WAL", err)
	}
	child.Process.Kill()
	child.Wait()
	recovered, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	checkRecovered := func(s *Store) {
		t.Helper()
		var integrity string
		if err := s.db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil || integrity != "ok" {
			t.Fatal(integrity, err)
		}
		feeds, err := s.Feeds(ctx, false)
		if err != nil || len(feeds) != 1 || !feeds[0].Subscribed {
			t.Fatal(feeds, err)
		}
		queue, err := s.Queue(ctx)
		if err != nil || len(queue) != 2 || !queue[0].Completed || queue[1].PositionSeconds != 42.5 {
			t.Fatal(queue, err)
		}
		current, err := s.Current(ctx)
		if err != nil || current.PositionSeconds != 42.5 {
			t.Fatal(current, err)
		}
	}
	checkRecovered(recovered)
	// VACUUM INTO reads a transactionally consistent snapshot including WAL.
	// A securely precreated zero-length file avoids SQLite's default permissions.
	backup := filepath.Join(t.TempDir(), "backup.db")
	f, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = recovered.db.Exec("VACUUM INTO ?", backup); err != nil {
		t.Fatal(err)
	}
	if err = recovered.ReplaceQueue(ctx, nil); err != nil {
		t.Fatal(err)
	} // snapshot independence
	restored, err := Open(ctx, backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	checkRecovered(restored)
	if stat, err := os.Stat(backup); err != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("backup permissions", err)
	}
}
func TestFailedUpgradePreservesListeningDataAndSchema(t *testing.T) {
	s, _ := openStore(t)
	id, err := s.UpsertFeed(ctx, Feed{URL: "https://example.test/rss"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpsertEpisodes(ctx, id, []Episode{{GUID: "one", AudioURL: "https://example.test/one"}}); err != nil {
		t.Fatal(err)
	}
	episodes, err := s.Episodes(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Checkpoint(ctx, episodes[0].ID, 123); err != nil {
		t.Fatal(err)
	}
	err = migrate(ctx, s.db, []string{schema, "ALTER TABLE episodes ADD COLUMN future TEXT; UPDATE playback SET position_seconds=0; INVALID SQL"})
	if err == nil {
		t.Fatal("broken upgrade succeeded")
	}
	var version int
	if err = s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatal(version, err)
	}
	state, err := s.Current(ctx)
	if err != nil || state.PositionSeconds != 123 {
		t.Fatal("upgrade corrupted listening data", state, err)
	}
	if _, err = s.db.Exec("SELECT future FROM episodes"); err == nil {
		t.Fatal("partial DDL survived rollback")
	}
}
