package store

import (
	"context"
	"database/sql"
	"fmt"
)

// EnqueueLatest atomically appends at most the newest cached episode of each
// subscribed feed and consumes a durable receipt even for current/completed or
// already-queued episodes. No older fallback or autoplay. feedID=0 means all.
func (s *Store) EnqueueLatest(ctx context.Context, currentID, feedID int64) ([]Episode, error) {
	if feedID < 0 {
		return nil, fmt.Errorf("store: invalid feed ID")
	}
	var pending []Episode
	err := transaction(ctx, s.db, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT e.id,COALESCE(p.completed,0) FROM feeds f
   JOIN episodes e ON e.id=(SELECT newest.id FROM episodes newest
    WHERE newest.feed_id=f.id ORDER BY newest.published_at DESC,newest.identity ASC LIMIT 1)
   LEFT JOIN playback p ON p.episode_id=e.id
   WHERE f.subscribed=1 AND (?=0 OR f.id=?) ORDER BY f.id`, feedID, feedID)
		if err != nil {
			return err
		}
		type candidate struct {
			id        int64
			completed bool
		}
		var candidates []candidate
		for rows.Next() {
			var c candidate
			if err := rows.Scan(&c.id, &c.completed); err != nil {
				rows.Close()
				return err
			}
			candidates = append(candidates, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, c := range candidates {
			receipt, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO autoqueue_seen(episode_id) VALUES(?)`, c.id)
			if err != nil {
				return err
			}
			count, err := receipt.RowsAffected()
			if err != nil {
				return err
			}
			if count == 0 || c.completed || c.id == currentID {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO queue(episode_id,ordinal)
    SELECT ?,COALESCE(MAX(ordinal)+1,0) FROM queue`, c.id); err != nil {
				return err
			}
		}
		rows, err = tx.QueryContext(ctx, `SELECT `+episodeColumns+` FROM `+episodeTables+` JOIN queue q ON q.episode_id=e.id ORDER BY q.ordinal`)
		if err != nil {
			return err
		}
		pending, err = readEpisodes(rows)
		return err
	})
	return pending, err
}
