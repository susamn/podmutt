// Package library provides cached subscription views and explicit user edits.
package library

import (
	"context"
	"slices"
	"sort"
	"strings"

	"podmutt/internal/feed"
	"podmutt/internal/httpurl"
	"podmutt/internal/store"
)

type Repository interface {
	Feeds(context.Context, bool) ([]store.Feed, error)
	Episodes(context.Context, int64) ([]store.Episode, error)
	InProgress(context.Context) ([]store.Episode, error)
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
	Feeds                    []store.Feed
	New, Continue, All, Show []store.Episode
}

type View int

const (
	ShowView View = iota
	NewView
	ContinueView
	AllView
)

func (v View) String() string {
	return [...]string{"Show", "Not started", "Continue listening", "All episodes"}[v]
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
	result.All = all
	result.Continue, err = s.repo.InProgress(ctx)
	if err != nil {
		return result, err
	}
	result.New = notStarted(all)
	if feedID > 0 {
		result.Show, err = s.repo.Episodes(ctx, feedID)
	}
	return result, err
}
func notStarted(episodes []store.Episode) []store.Episode {
	var result []store.Episode
	for _, episode := range episodes {
		if !episode.Completed && episode.PositionSeconds == 0 {
			result = append(result, episode)
		}
	}
	return result
}

// WithCurrent overlays live progress before the next saved checkpoint. Copies
// keep cached snapshots immutable; unsubscribed current audio belongs only in
// Continue, never in the subscribed All/Not started collections.
func (snapshot Snapshot) WithCurrent(current *store.Episode) Snapshot {
	if current == nil {
		return snapshot
	}
	replace := func(episodes []store.Episode) []store.Episode {
		result := slices.Clone(episodes)
		for i := range result {
			if result[i].ID == current.ID {
				result[i] = *current
			}
		}
		return result
	}
	snapshot.All, snapshot.Show = replace(snapshot.All), replace(snapshot.Show)
	snapshot.New = notStarted(snapshot.All)
	snapshot.Continue = slices.DeleteFunc(slices.Clone(snapshot.Continue), func(episode store.Episode) bool { return episode.ID == current.ID })
	if !current.Completed && current.PositionSeconds > 0 {
		snapshot.Continue = append(snapshot.Continue, *current)
	}
	return snapshot
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
			if episodes, err := s.repo.Episodes(ctx, source.ID); err == nil && len(episodes) > 0 {
				return source.ID, s.Subscribe(ctx, source.ID, true)
			}
			break
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
func Episodes(snapshot Snapshot, view View, order Order) []store.Episode {
	source := snapshot.Show
	switch view {
	case NewView:
		source = snapshot.New
	case ContinueView:
		source = snapshot.Continue
	case AllView:
		source = snapshot.All
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
