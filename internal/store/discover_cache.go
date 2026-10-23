package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DiscoverSnapshot is an ordered, provider-specific result payload. The app
// persists only Apple snapshots; Podcast Index retention is header-governed.
type DiscoverSnapshot struct {
	Provider, Country, Query, Mode string
	Results                        []byte
	FetchedAt                      time.Time
}

// DiscoverView is navigation state, not a provider response cache.
type DiscoverView struct {
	Provider, Country, Query, Mode, SelectedFeedURL string
}

func normalizeDiscoverKey(provider, country, query, mode string) (string, string, string, string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	country = strings.ToLower(strings.TrimSpace(country))
	query = strings.Join(strings.Fields(query), " ")
	mode = strings.ToLower(strings.TrimSpace(mode))
	if provider != "apple" {
		return "", "", "", "", fmt.Errorf("provider does not permit durable snapshots")
	}
	if country == "" {
		country = "us"
	}
	if mode != "chart" && mode != "search" {
		return "", "", "", "", fmt.Errorf("invalid Discover mode")
	}
	if mode == "chart" {
		query = ""
	}
	if mode == "search" && query == "" {
		return "", "", "", "", fmt.Errorf("search snapshot requires a query")
	}
	return provider, country, query, mode, nil
}

func (s *Store) SaveDiscoverSnapshot(ctx context.Context, snapshot DiscoverSnapshot) error {
	provider, country, query, mode, err := normalizeDiscoverKey(snapshot.Provider, snapshot.Country, snapshot.Query, snapshot.Mode)
	if err != nil {
		return err
	}
	if len(snapshot.Results) == 0 || len(snapshot.Results) > 1<<20 {
		return fmt.Errorf("invalid Discover snapshot size")
	}
	if snapshot.FetchedAt.IsZero() {
		snapshot.FetchedAt = time.Now().UTC()
	}
	return transaction(ctx, s.db, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO discover_snapshots(provider,country,query,mode,results_json,fetched_at)
			VALUES(?,?,?,?,?,?) ON CONFLICT(provider,country,query,mode) DO UPDATE SET
			results_json=excluded.results_json,fetched_at=excluded.fetched_at`, provider, country, query, mode, string(snapshot.Results), snapshot.FetchedAt.Unix())
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM discover_snapshots WHERE rowid IN (
			SELECT rowid FROM discover_snapshots ORDER BY fetched_at DESC LIMIT -1 OFFSET 24)`)
		return err
	})
}

func (s *Store) LoadDiscoverSnapshot(ctx context.Context, provider, country, query, mode string) (DiscoverSnapshot, error) {
	provider, country, query, mode, err := normalizeDiscoverKey(provider, country, query, mode)
	if err != nil {
		return DiscoverSnapshot{}, err
	}
	var snapshot DiscoverSnapshot
	var data string
	var fetched int64
	err = s.db.QueryRowContext(ctx, `SELECT provider,country,query,mode,results_json,fetched_at FROM discover_snapshots
		WHERE provider=? AND country=? AND query=? AND mode=?`, provider, country, query, mode).Scan(&snapshot.Provider, &snapshot.Country, &snapshot.Query, &snapshot.Mode, &data, &fetched)
	if err != nil {
		return DiscoverSnapshot{}, err
	}
	snapshot.Results = []byte(data)
	snapshot.FetchedAt = time.Unix(fetched, 0).UTC()
	return snapshot, nil
}

func (s *Store) SaveDiscoverView(ctx context.Context, view DiscoverView) error {
	view.Provider = strings.ToLower(strings.TrimSpace(view.Provider))
	view.Country = strings.ToLower(strings.TrimSpace(view.Country))
	view.Query = strings.Join(strings.Fields(view.Query), " ")
	view.Mode = strings.ToLower(strings.TrimSpace(view.Mode))
	if view.Provider != "apple" && view.Provider != "podcast_index" {
		return fmt.Errorf("invalid Discover provider")
	}
	if view.Country == "" && view.Provider == "apple" {
		view.Country = "us"
	}
	if view.Mode != "chart" && view.Mode != "search" {
		return fmt.Errorf("invalid Discover mode")
	}
	if view.Mode == "chart" {
		view.Query = ""
	}
	if view.Provider == "podcast_index" {
		view.SelectedFeedURL = ""
	}
	if view.Mode == "search" && view.Query == "" {
		return fmt.Errorf("search view requires a query")
	}
	if len(view.SelectedFeedURL) > 2048 {
		return fmt.Errorf("selected feed URL too long")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO discover_view(singleton,provider,country,query,mode,selected_feed_url)
		VALUES(1,?,?,?,?,?) ON CONFLICT(singleton) DO UPDATE SET provider=excluded.provider,country=excluded.country,
		query=excluded.query,mode=excluded.mode,selected_feed_url=excluded.selected_feed_url,updated_at=strftime('%s','now')`,
		view.Provider, view.Country, view.Query, view.Mode, view.SelectedFeedURL)
	return err
}

func (s *Store) LoadDiscoverView(ctx context.Context) (DiscoverView, error) {
	var view DiscoverView
	err := s.db.QueryRowContext(ctx, `SELECT provider,country,query,mode,selected_feed_url FROM discover_view WHERE singleton=1`).Scan(
		&view.Provider, &view.Country, &view.Query, &view.Mode, &view.SelectedFeedURL)
	return view, err
}
