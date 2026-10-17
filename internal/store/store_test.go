package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var ctx = context.Background()

func openStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "podcasts.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestOpenAndReopen(t *testing.T) {
	s, path := openStore(t)
	var version, foreignKeys int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("version = %d, err = %v", version, err)
	}
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", foreignKeys, err)
	}
	var journal string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil || journal != "wal" {
		t.Fatalf("journal_mode = %q, err = %v", journal, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("database permissions: %v, %v", info, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.db.Exec("INSERT INTO playback(episode_id) VALUES (999)"); err == nil {
		t.Fatal("foreign key violation accepted")
	}
}

func TestOpenErrors(t *testing.T) {
	dir := t.TempDir()
	public := filepath.Join(dir, "public.db")
	broken := filepath.Join(dir, "broken.db")
	link := filepath.Join(dir, "link.db")
	for _, path := range []string{public, broken} {
		if err := os.WriteFile(path, []byte("not sqlite"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(public, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(broken, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative.db", filepath.Join(dir, "absent", "file.db"), dir, public, broken, link} {
		t.Run(path, func(t *testing.T) {
			if s, err := Open(ctx, path); err == nil {
				s.Close()
				t.Fatal("expected error")
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Open(cancelled, filepath.Join(dir, "cancelled.db")); err == nil {
		t.Fatal("cancelled open succeeded")
	}
}

func TestMigrationRollbackAndUpgrade(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	first := "CREATE TABLE test (id INTEGER PRIMARY KEY)"
	if err := migrate(ctx, db, []string{first, "INVALID SQL"}); err == nil {
		t.Fatal("expected migration failure")
	}
	var version, count int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 0 {
		t.Fatalf("failed migration changed version: %d, %v", version, err)
	}
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = 'test'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed migration left schema: %d, %v", count, err)
	}
	if err := migrate(ctx, db, []string{first}); err != nil {
		t.Fatal(err)
	}
	upgrade := []string{first, "ALTER TABLE test ADD COLUMN name TEXT"}
	for range 2 {
		if err := migrate(ctx, db, upgrade); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrate(ctx, db, []string{first}); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected downgrade rejection: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = -1"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db, upgrade); err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("expected invalid schema version rejection: %v", err)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	s, path := openStore(t)
	if _, err := s.db.Exec("PRAGMA user_version = 100"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if _, err := Open(ctx, path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("expected newer schema rejection: %v", err)
	}
}
