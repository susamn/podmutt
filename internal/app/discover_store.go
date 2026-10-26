package app

import (
	"context"
	"encoding/json"
	"time"

	"podmutt/internal/discovery"
	"podmutt/internal/store"
)

// discoverStore encapsulates SQLite persistence for discovery snapshots,
// view preferences, discovered show metadata, and customer star ratings.

func loadCachedDiscoverSnapshot(ctx context.Context, repo *store.Store, country, query, mode string) ([]discovery.Show, time.Time, bool) {
	if repo == nil {
		return nil, time.Time{}, false
	}
	snapshot, err := repo.LoadDiscoverSnapshot(ctx, "apple", country, query, mode)
	if err != nil {
		return nil, time.Time{}, false
	}
	var cached []discovery.Show
	if err := json.Unmarshal(snapshot.Results, &cached); err != nil {
		return nil, time.Time{}, false
	}
	return cached, snapshot.FetchedAt, true
}

func saveDiscoverView(ctx context.Context, repo *store.Store, view store.DiscoverView) error {
	if repo == nil {
		return nil
	}
	return repo.SaveDiscoverView(ctx, view)
}

func persistDiscoverSnapshot(ctx context.Context, repo *store.Store, country, query, mode, selectedURL string, shows []discovery.Show, savedAt time.Time) error {
	if repo == nil {
		return nil
	}
	results, err := json.Marshal(shows)
	if err != nil {
		return err
	}
	if err := repo.SaveDiscoverSnapshot(ctx, store.DiscoverSnapshot{
		Provider:  "apple",
		Country:   country,
		Query:     query,
		Mode:      mode,
		Results:   results,
		FetchedAt: savedAt,
	}); err != nil {
		return err
	}
	if err := repo.SaveDiscoverView(ctx, store.DiscoverView{
		Provider:        "apple",
		Country:         country,
		Query:           query,
		Mode:            mode,
		SelectedFeedURL: selectedURL,
	}); err != nil {
		return err
	}
	if len(shows) > 0 {
		sourceID, err := repo.SourceID(ctx, "apple")
		if err == nil && sourceID > 0 {
			var toStore []store.DiscoveredShow
			for _, show := range shows {
				toStore = append(toStore, store.DiscoveredShow{
					Title:            show.Title,
					FeedURL:          show.FeedURL,
					ArtworkURL:       show.SourceURL,
					Country:          country,
					SourceID:         sourceID,
					SourceExternalID: show.ExternalID,
				})
			}
			_ = repo.UpsertDiscoveredShows(ctx, country, sourceID, toStore)
		}
	}
	return nil
}

func cachedShowRating(ctx context.Context, repo *store.Store, country, title, feedURL string) (float64, int, bool) {
	if repo == nil {
		return 0, 0, false
	}
	shows, err := repo.DiscoveredShows(ctx, country, title, 5)
	if err != nil {
		return 0, 0, false
	}
	for _, s := range shows {
		if s.FeedURL == feedURL && s.Rating > 0 {
			return s.Rating, s.ReviewCount, true
		}
	}
	return 0, 0, false
}

func persistShowRating(ctx context.Context, repo *store.Store, country, feedURL string, rating float64, count int) error {
	if repo == nil || rating <= 0 {
		return nil
	}
	sourceID, err := repo.SourceID(ctx, "apple")
	if err != nil || sourceID <= 0 {
		return err
	}
	shows, err := repo.DiscoveredShows(ctx, country, "", 100)
	if err != nil {
		return err
	}
	for _, s := range shows {
		if s.FeedURL == feedURL {
			return repo.UpsertShowRating(ctx, s.ID, sourceID, rating, count)
		}
	}
	return nil
}
