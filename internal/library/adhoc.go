package library

import (
	"context"
	"fmt"
	"strings"

	"podmutt/internal/feed"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

// CachePreviewEpisodes materializes explicit user selections without subscribing,
// refetching or advancing feed validators (only part of the feed is cached).
func (s *Service) CachePreviewEpisodes(ctx context.Context, preview feed.Resolved, indices []int) ([]int64, error) {
	address, err := httpurl.Parse(preview.URL)
	if err != nil {
		return nil, err
	}
	address.Fragment = ""
	if preview.Fetched.NotModified || len(indices) == 0 {
		return nil, fmt.Errorf("select an episode from a loaded preview")
	}
	selected := make([]store.Episode, 0, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(preview.Fetched.Document.Episodes) {
			return nil, fmt.Errorf("preview episode is no longer available")
		}
		episode := preview.Fetched.Document.Episodes[index]
		if _, err := httpurl.Parse(episode.AudioURL); err != nil {
			return nil, err
		}
		selected = append(selected, episode)
	}
	sources, err := s.repo.Feeds(ctx, false)
	if err != nil {
		return nil, err
	}
	id := int64(0)
	for _, source := range sources {
		if source.URL == address.String() {
			id = source.ID
			break
		}
	}
	if id == 0 {
		id, err = s.repo.UpsertFeed(ctx, store.Feed{URL: address.String(), Title: preview.Fetched.Document.Title, ArtworkURL: preview.Fetched.Document.ArtworkURL})
		if err != nil {
			return nil, err
		}
	}
	if err := s.repo.UpsertEpisodes(ctx, id, selected); err != nil {
		return nil, err
	}
	cached, err := s.repo.Episodes(ctx, id)
	if err != nil {
		return nil, err
	}
	identities := make(map[string]int64, len(cached))
	for _, episode := range cached {
		identities[episodeIdentity(episode)] = episode.ID
	}
	ids := make([]int64, 0, len(selected))
	for _, episode := range selected {
		localID, ok := identities[episodeIdentity(episode)]
		if !ok {
			return nil, fmt.Errorf("preview episode could not be cached")
		}
		ids = append(ids, localID)
	}
	return ids, nil
}
func episodeIdentity(episode store.Episode) string {
	if guid := strings.TrimSpace(episode.GUID); guid != "" {
		return "guid:" + guid
	}
	return "url:" + episode.AudioURL
}
