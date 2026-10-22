package library

import (
	"context"
	"strings"

	"podmutt/internal/feed"
	"podmutt/internal/feedback"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

// Preview does not store a show, change subscription/history, or fetch audio.
func (s *Service) Preview(ctx context.Context, raw string) (feed.Resolved, error) {
	address, err := httpurl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return feed.Resolved{}, err
	}
	address.Fragment = ""
	var result feed.Resolved
	if resolver, ok := s.fetcher.(interface {
		Resolve(context.Context, string) (feed.Resolved, error)
	}); ok {
		result, err = resolver.Resolve(ctx, address.String())
	} else {
		result.URL = address.String()
		result.Fetched, err = s.fetcher.Fetch(ctx, store.Feed{URL: result.URL})
	}
	if err != nil {
		return feed.Resolved{}, err
	}
	if result.Fetched.NotModified {
		return feed.Resolved{}, feedback.New(feedback.InvalidResponse, "new feed returned no content")
	}
	return result, nil
}

// SubscribePreview commits validated preview metadata only after explicit user
// confirmation. Unsubscribed ad-hoc caches receive the full preview before
// subscription; existing episode IDs/history survive. Already subscribed URLs
// retain their cache without an unnecessary rewrite.
func (s *Service) SubscribePreview(ctx context.Context, preview feed.Resolved) (int64, error) {
	address, err := httpurl.Parse(preview.URL)
	if err != nil {
		return 0, err
	}
	address.Fragment = ""
	if preview.Fetched.NotModified {
		return 0, feedback.New(feedback.InvalidResponse, "preview has no content")
	}
	sources, err := s.repo.Feeds(ctx, false)
	if err != nil {
		return 0, err
	}
	for _, source := range sources {
		if source.URL == address.String() && source.Subscribed {
			return source.ID, nil
		}
	}
	fetched := preview.Fetched
	source := store.Feed{URL: address.String(), Title: fetched.Document.Title, ArtworkURL: fetched.Document.ArtworkURL}
	id, err := s.repo.UpsertFeed(ctx, source)
	if err != nil {
		return 0, err
	}
	if err := s.repo.UpsertEpisodes(ctx, id, fetched.Document.Episodes); err != nil {
		return 0, err
	}
	source.ETag, source.LastModified = fetched.ETag, fetched.LastModified
	if _, err := s.repo.UpsertFeed(ctx, source); err != nil {
		return 0, err
	}
	return id, s.Subscribe(ctx, id, true)
}
