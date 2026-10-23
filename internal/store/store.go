// Package store owns local podcast metadata and listening state, independent of
// the UI and player. Callers must hold profile ownership before writing.
package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"podmutt/internal/feedback"
	"syscall"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

//go:embed autoqueue.sql
var autoqueueSchema string

//go:embed discovery.sql
var discoverySchema string

//go:embed favorites.sql
var favoritesSchema string

//go:embed chapters.sql
var chaptersSchema string

//go:embed chapters_origin.sql
var chaptersOriginSchema string

var migrations = []string{schema, autoqueueSchema, discoverySchema, favoritesSchema, chaptersSchema, chaptersOriginSchema}

// Store serializes database access through one connection. The profile directory
// must already exist and be private; profile.Paths.Prepare establishes that.
type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, feedback.New(feedback.Profile, "store: database path must be absolute")
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	file.Close()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return nil, feedback.New(feedback.Profile, "store: database must be a private regular file owned by current user")
	}
	uri := url.URL{Scheme: "file", Path: path}
	query := uri.Query()
	for _, pragma := range []string{"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)"} {
		query.Add("_pragma", pragma)
	}
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(ctx, db, migrations); err != nil {
		db.Close()
		var classified *feedback.Fault
		if errors.As(err, &classified) {
			return nil, err
		}
		return nil, feedback.Wrap(feedback.Database, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func transaction(ctx context.Context, db *sql.DB, apply func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := apply(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func migrate(ctx context.Context, db *sql.DB, migrations []string) error {
	return transaction(ctx, db, func(tx *sql.Tx) error {
		var version int
		if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
			return err
		}
		if version < 0 {
			return fmt.Errorf("invalid negative database version %d", version)
		}
		if version > len(migrations) {
			return feedback.New(feedback.NewerDatabase, fmt.Sprintf("Database version %d is newer than supported version %d.", version, len(migrations)))
		}
		for i := version; i < len(migrations); i++ {
			if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
				return err
			}
		}
		return nil
	})
}
