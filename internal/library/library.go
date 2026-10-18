// Package library provides cached subscription views and explicit user edits.
package library

import (
	"context"
	"sort"
	"strings"

	"podmutt/internal/feed"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

type Repository interface {
	Feeds(context.Context, bool) ([]store.Feed, error)
	Episodes(context.Context, int64) ([]store.Episode, error)
	UpsertFeed(context.Context, store.Feed) (int64, error)
	UpsertEpisodes(context.Context, int64, []store.Episode) error
	SetSubscribed(context.Context, int64, bool) error
}
type Fetcher interface {
	Fetch(context.Context, store.Feed) (feed.Fetched, error)
}
type Service struct {
	repo    Repository
	fetcher Fetcher
}

func New(repo Repository, fetcher Fetcher) *Service { return &Service{repo, fetcher} }

type Snapshot struct {
	Feeds     []store.Feed
	New, Show []store.Episode
}

func (s *Service) Read(ctx context.Context, feedID int64) (Snapshot, error) {
	var result Snapshot
	var err error
	result.Feeds, err = s.repo.Feeds(ctx, false)
	if err != nil {
		return result, err
	}
	all, err := s.repo.Episodes(ctx, 0)
	if err != nil {
		return result, err
	}
	for _, episode := range all {
		if !episode.Completed {
			result.New = append(result.New, episode)
		}
	}
	if feedID > 0 {
		result.Show, err = s.repo.Episodes(ctx, feedID)
	}
	return result, err
}
func (s *Service) Subscribe(ctx context.Context, id int64, subscribed bool) error {
	return s.repo.SetSubscribed(ctx, id, subscribed)
}

// Add validates/fetches before creating a row. Existing URLs resubscribe without
// overwriting metadata/history; new feeds become subscribed only after caching.
func (s *Service) Add(ctx context.Context, raw string) (int64, error) {
	address, err := httpurl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	address.Fragment = ""
	sources, err := s.repo.Feeds(ctx, false)
	if err != nil {
		return 0, err
	}
	for _, source := range sources {
		if source.URL == address.String() {
			return source.ID, s.Subscribe(ctx, source.ID, true)
		}
	}
	preview, err := s.Preview(ctx, address.String())
	if err != nil {
		return 0, err
	}
	return s.SubscribePreview(ctx, preview)
}

type Order int

const (
	Newest Order = iota
	Oldest
	Title
)

func (o Order) String() string { return [...]string{"newest", "oldest", "title"}[o] }

// Episodes returns a sorted copy, so changing tabs/order never mutates a snapshot.
func Episodes(snapshot Snapshot, newOnly bool, order Order) []store.Episode {
	source := snapshot.Show
	if newOnly {
		source = snapshot.New
	}
	result := append([]store.Episode(nil), source...)
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if order == Title {
			left, right := strings.ToLower(PlainText(a.Title)), strings.ToLower(PlainText(b.Title))
			if left != right {
				return left < right
			}
		} else if !a.PublishedAt.Equal(b.PublishedAt) {
			if order == Oldest {
				return a.PublishedAt.Before(b.PublishedAt)
			}
			return a.PublishedAt.After(b.PublishedAt)
		}
		return a.ID < b.ID
	})
	return result
}
