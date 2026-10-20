package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type DiscoveredShow struct {
	ID               int64
	Title            string
	FeedURL          string
	ArtworkURL       string
	Description      string
	Country          string
	SourceID         int64
	SourceName       string
	SourceExternalID string
	Rating           float64
	ReviewCount      int
	UpdatedAt        time.Time
}

type ShowRating struct {
	ShowID      int64
	SourceID    int64
	SourceName  string
	Rating      float64
	ReviewCount int
	UpdatedAt   time.Time
}

// SourceID looks up a source ID by its unique canonical name (e.g. "apple", "podcast_index").
func (s *Store) SourceID(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM sources WHERE name = ?`, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("store: source %q not found", name)
	}
	return id, err
}

// UpsertDiscoveredShows inserts or updates discovered podcasts in bulk, preserving
// existing ratings and updating modified timestamps.
func (s *Store) UpsertDiscoveredShows(ctx context.Context, country string, sourceID int64, shows []DiscoveredShow) error {
	if country == "" {
		country = "us"
	}
	return transaction(ctx, s.db, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO discovered_shows (title, feed_url, artwork_url, description, country, source_id, source_external_id, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'))
			ON CONFLICT(feed_url) DO UPDATE SET
				title = CASE WHEN excluded.title != '' THEN excluded.title ELSE discovered_shows.title END,
				artwork_url = CASE WHEN excluded.artwork_url != '' THEN excluded.artwork_url ELSE discovered_shows.artwork_url END,
				description = CASE WHEN excluded.description != '' THEN excluded.description ELSE discovered_shows.description END,
				country = excluded.country,
				source_id = excluded.source_id,
				source_external_id = CASE WHEN excluded.source_external_id != '' THEN excluded.source_external_id ELSE discovered_shows.source_external_id END,
				updated_at = strftime('%s', 'now')
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, show := range shows {
			if strings.TrimSpace(show.FeedURL) == "" {
				continue
			}
			showCountry := show.Country
			if showCountry == "" {
				showCountry = country
			}
			srcID := show.SourceID
			if srcID == 0 {
				srcID = sourceID
			}
			if _, err := stmt.ExecContext(ctx, show.Title, show.FeedURL, show.ArtworkURL, show.Description, showCountry, srcID, show.SourceExternalID); err != nil {
				return err
			}
		}
		return nil
	})
}

// DiscoveredShows searches locally cached discovery shows matching country and an optional title/query substring.
func (s *Store) DiscoveredShows(ctx context.Context, country, query string, limit int) ([]DiscoveredShow, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	query = strings.TrimSpace(query)
	var rows *sql.Rows
	var err error

	baseQuery := `
		SELECT d.id, d.title, d.feed_url, d.artwork_url, d.description, d.country, d.source_id,
		       s.name, d.source_external_id, COALESCE(r.rating, 0), COALESCE(r.review_count, 0), d.updated_at
		FROM discovered_shows d
		JOIN sources s ON s.id = d.source_id
		LEFT JOIN show_ratings r ON r.show_id = d.id
	`

	if query == "" {
		if country != "" {
			rows, err = s.db.QueryContext(ctx, baseQuery+` WHERE d.country = ? ORDER BY d.updated_at DESC, d.id ASC LIMIT ?`, country, limit)
		} else {
			rows, err = s.db.QueryContext(ctx, baseQuery+` ORDER BY d.updated_at DESC, d.id ASC LIMIT ?`, limit)
		}
	} else {
		like := "%" + query + "%"
		if country != "" {
			rows, err = s.db.QueryContext(ctx, baseQuery+` WHERE (d.country = ? OR d.country = 'us') AND (d.title LIKE ? OR d.description LIKE ?) ORDER BY d.updated_at DESC, d.id ASC LIMIT ?`, country, like, like, limit)
		} else {
			rows, err = s.db.QueryContext(ctx, baseQuery+` WHERE d.title LIKE ? OR d.description LIKE ? ORDER BY d.updated_at DESC, d.id ASC LIMIT ?`, like, like, limit)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DiscoveredShow
	for rows.Next() {
		var item DiscoveredShow
		var updated int64
		if err := rows.Scan(&item.ID, &item.Title, &item.FeedURL, &item.ArtworkURL, &item.Description, &item.Country, &item.SourceID,
			&item.SourceName, &item.SourceExternalID, &item.Rating, &item.ReviewCount, &updated); err != nil {
			return nil, err
		}
		item.UpdatedAt = time.Unix(updated, 0)
		result = append(result, item)
	}
	return result, rows.Err()
}

// UpsertShowRating sets or updates a rating entry for a show from a given source.
func (s *Store) UpsertShowRating(ctx context.Context, showID int64, sourceID int64, rating float64, reviewCount int) error {
	if rating < 0 || rating > 5 {
		return fmt.Errorf("store: rating must be between 0 and 5, got %v", rating)
	}
	if reviewCount < 0 {
		reviewCount = 0
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO show_ratings (show_id, source_id, rating, review_count, updated_at)
		VALUES (?, ?, ?, ?, strftime('%s', 'now'))
		ON CONFLICT(show_id, source_id) DO UPDATE SET
			rating = excluded.rating,
			review_count = excluded.review_count,
			updated_at = strftime('%s', 'now')
	`, showID, sourceID, rating, reviewCount)
	return err
}

// ShowRatings returns all source ratings recorded for a discovered show.
func (s *Store) ShowRatings(ctx context.Context, showID int64) ([]ShowRating, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.show_id, r.source_id, s.name, r.rating, r.review_count, r.updated_at
		FROM show_ratings r
		JOIN sources s ON s.id = r.source_id
		WHERE r.show_id = ?
		ORDER BY r.source_id ASC
	`, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ratings []ShowRating
	for rows.Next() {
		var item ShowRating
		var updated int64
		if err := rows.Scan(&item.ShowID, &item.SourceID, &item.SourceName, &item.Rating, &item.ReviewCount, &updated); err != nil {
			return nil, err
		}
		item.UpdatedAt = time.Unix(updated, 0)
		ratings = append(ratings, item)
	}
	return ratings, rows.Err()
}
