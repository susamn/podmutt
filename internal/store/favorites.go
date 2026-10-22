package store

import (
	"context"
	"fmt"
)

// AddFavorite idempotently records an episode as a favorite.
// Foreign key constraints require the episode to exist in the store.
func (s *Store) AddFavorite(ctx context.Context, episodeID int64) error {
	if episodeID <= 0 {
		return fmt.Errorf("store: invalid episode ID for favorite")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO favorites(episode_id) VALUES(?)
		ON CONFLICT(episode_id) DO NOTHING`, episodeID)
	return err
}

// RemoveFavorite removes an episode from favorites if present.
func (s *Store) RemoveFavorite(ctx context.Context, episodeID int64) error {
	if episodeID <= 0 {
		return fmt.Errorf("store: invalid episode ID for favorite")
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM favorites WHERE episode_id=?", episodeID)
	return err
}

// IsFavorite reports whether an episode ID is currently favorited.
func (s *Store) IsFavorite(ctx context.Context, episodeID int64) (bool, error) {
	if episodeID <= 0 {
		return false, nil
	}
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM favorites WHERE episode_id=?)", episodeID).Scan(&exists)
	return exists, err
}

// FavoriteEpisodes returns all favorited episodes ordered by addition time (newest first).
func (s *Store) FavoriteEpisodes(ctx context.Context) ([]Episode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+episodeColumns+` FROM `+episodeTables+`
		JOIN favorites fav ON fav.episode_id=e.id
		ORDER BY fav.id DESC`)
	if err != nil {
		return nil, err
	}
	return readEpisodes(rows)
}

// FavoriteIDs returns the set of all favorited episode IDs for O(1) membership checks.
func (s *Store) FavoriteIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT episode_id FROM favorites")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}
