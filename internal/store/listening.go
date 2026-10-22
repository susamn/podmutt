package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

type Playback struct {
	EpisodeID       int64
	PositionSeconds float64
	Completed       bool
}

// Checkpoint atomically changes the current episode and saves its position.
// It does not infer completion or reset an existing completion mark.
func (s *Store) Checkpoint(ctx context.Context, episodeID int64, seconds float64) error {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("store: position must be finite and nonnegative")
	}
	return transaction(ctx, s.db, func(tx *sql.Tx) error { return checkpoint(ctx, tx, episodeID, seconds) })
}

func checkpoint(ctx context.Context, tx *sql.Tx, episodeID int64, seconds float64) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO playback(episode_id,position_seconds) VALUES(?,?)
			ON CONFLICT(episode_id) DO UPDATE SET position_seconds=excluded.position_seconds`, episodeID, seconds); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO current_episode(singleton,episode_id) VALUES(1,?)
			ON CONFLICT(singleton) DO UPDATE SET episode_id=excluded.episode_id`, episodeID)
	return err
}

func (s *Store) SetCompleted(ctx context.Context, episodeID int64, completed bool) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO playback(episode_id,completed) VALUES(?,?)
		ON CONFLICT(episode_id) DO UPDATE SET completed=excluded.completed`, episodeID, completed)
	return err
}

// Playback returns zero progress for a known episode without history, and
// sql.ErrNoRows for an unknown episode.
func (s *Store) Playback(ctx context.Context, episodeID int64) (Playback, error) {
	var playback Playback
	err := s.db.QueryRowContext(ctx, `SELECT e.id,COALESCE(p.position_seconds,0),COALESCE(p.completed,0)
		FROM `+episodeTables+` WHERE e.id=?`, episodeID).
		Scan(&playback.EpisodeID, &playback.PositionSeconds, &playback.Completed)
	return playback, err
}

// Current returns sql.ErrNoRows when playback has never been checkpointed.
func (s *Store) Current(ctx context.Context) (Playback, error) {
	var playback Playback
	err := s.db.QueryRowContext(ctx, `SELECT p.episode_id,p.position_seconds,p.completed
		FROM current_episode c JOIN playback p ON p.episode_id=c.episode_id WHERE c.singleton=1`).
		Scan(&playback.EpisodeID, &playback.PositionSeconds, &playback.Completed)
	return playback, err
}

// ClearCurrent removes only the captured current pointer, retaining all history.
// A stale identity cannot clear a different episode.
func (s *Store) ClearCurrent(ctx context.Context, episodeID int64) error {
	var removed int64
	return s.db.QueryRowContext(ctx, "DELETE FROM current_episode WHERE singleton=1 AND episode_id=? RETURNING episode_id", episodeID).Scan(&removed)
}

// ReplaceQueue deduplicates in first-occurrence order. Unknown episode IDs cause
// the whole replacement to roll back. A nil/empty slice clears the queue.
func (s *Store) ReplaceQueue(ctx context.Context, episodeIDs []int64) error {
	return transaction(ctx, s.db, func(tx *sql.Tx) error { return replaceQueue(ctx, tx, episodeIDs) })
}

func replaceQueue(ctx context.Context, tx *sql.Tx, episodeIDs []int64) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM queue"); err != nil {
		return err
	}
	seen := make(map[int64]bool, len(episodeIDs))
	for _, id := range episodeIDs {
		if seen[id] {
			continue
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO queue(episode_id,ordinal) VALUES(?,?)", id, len(seen)); err != nil {
			return err
		}
		seen[id] = true
	}
	return nil
}

// Activate atomically promotes an episode and saves the remaining playback
// stack. Failed queue validation rolls back history/current/completion together.
func (s *Store) Activate(ctx context.Context, episodeID int64, seconds float64, pending []int64) error {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return fmt.Errorf("store: position must be finite and nonnegative")
	}
	return transaction(ctx, s.db, func(tx *sql.Tx) error {
		if err := checkpoint(ctx, tx, episodeID, seconds); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE playback SET completed=0 WHERE episode_id=?`, episodeID); err != nil {
			return err
		}
		return replaceQueue(ctx, tx, pending)
	})
}

func (s *Store) Queue(ctx context.Context) ([]Episode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+episodeColumns+` FROM `+episodeTables+`
		JOIN queue q ON q.episode_id=e.id ORDER BY q.ordinal`)
	if err != nil {
		return nil, err
	}
	return readEpisodes(rows)
}
