package app

import (
	"context"
	"testing"
	"time"

	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestDiscoverPersistenceCacheWriteFailureResilience(t *testing.T) {
	// A nil repository must not crash or discard in-memory shows
	err := persistDiscoverSnapshot(context.Background(), nil, "us", "tech", "search", "", []discovery.Show{{Title: "Tech Show"}}, time.Now())
	if err != nil {
		t.Fatalf("expected nil error for nil repository, got %v", err)
	}

	err = persistShowRating(context.Background(), nil, "us", "https://example.com/feed", 4.5, 100)
	if err != nil {
		t.Fatalf("expected nil error for nil repository, got %v", err)
	}
}

func TestDiscoverPersistencePodcastIndexNotCached(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		app.browse.active = true
		app.browse.usingIndex = true
		app.browse.generation = 1

		// Apply directory results from Podcast Index
		app.applyDirectory(directoryLoaded{
			generation: 1,
			provider:   "podcast_index",
			country:    "us",
			query:      "linux",
			mode:       "search",
			shows:      []discovery.Show{{Title: "Linux Show", FeedURL: "https://example.com/linux.xml"}},
		})

		// Shows must be present in memory for UI projection
		if len(app.browse.shows) != 1 || app.browse.shows[0].Title != "Linux Show" {
			t.Fatalf("expected 1 in-memory show, got %+v", app.browse.shows)
		}

		// Podcast Index results must NOT be persisted to the snapshot cache
		_, err = s.store.LoadDiscoverSnapshot(s.ctx, "podcast_index", "us", "linux", "search")
		if err == nil {
			t.Fatal("expected error loading unpersisted Podcast Index snapshot, got nil")
		}
	})
}

func TestDiscoverPersistenceRapidQueryCancellation(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		app.browse.active = true
		app.browse.country = "us"

		// Query 1 starts (generation 1)
		app.browse.generation = 1
		app.browse.query = "first"

		// Rapid query 2 supersedes it (cancels and increments generation to 2)
		app.cancelDirectory()
		if app.browse.generation != 2 {
			t.Fatalf("expected generation 2 after cancel, got %d", app.browse.generation)
		}
		app.browse.query = "second"
		app.browse.shows = []discovery.Show{{Title: "Newer Show", FeedURL: "https://example.com/2.xml"}}

		// Late-arriving response from generation 1 must be discarded
		app.applyDirectory(directoryLoaded{
			generation: 1,
			provider:   "apple",
			country:    "us",
			query:      "first",
			mode:       "search",
			shows:      []discovery.Show{{Title: "Stale Show", FeedURL: "https://example.com/1.xml"}},
		})

		if len(app.browse.shows) != 1 || app.browse.shows[0].Title != "Newer Show" {
			t.Fatalf("stale generation overwrote newer query results: %+v", app.browse.shows)
		}
	})
}

func TestDiscoverPersistenceSelectionRetention(t *testing.T) {
	librarySession(t, func(s *session) {
		app, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}

		app.browse.active = true
		app.browse.generation = 5
		app.browse.selectedFeedURL = "https://example.com/target.xml"
		app.Model = app.Model.WithLibrary(ui.Data{
			Podcasts: []ui.Item{{ID: "https://example.com/target.xml", Title: "Target Show"}},
		}).Select(ui.Podcasts, "https://example.com/target.xml")

		// Refresh arrives containing target show
		app.applyDirectory(directoryLoaded{
			generation: 5,
			provider:   "apple",
			country:    "us",
			query:      "",
			mode:       "chart",
			shows: []discovery.Show{
				{Title: "Other Show", FeedURL: "https://example.com/other.xml"},
				{Title: "Target Show", FeedURL: "https://example.com/target.xml"},
			},
		})

		if app.browse.selectedFeedURL != "https://example.com/target.xml" {
			t.Fatalf("expected selection to be retained, got %q", app.browse.selectedFeedURL)
		}
	})
}

func TestDiscoverPersistenceCachedRatingLookup(t *testing.T) {
	librarySession(t, func(s *session) {
		sourceID, err := s.store.SourceID(s.ctx, "apple")
		if err != nil {
			t.Fatal(err)
		}
		err = s.store.UpsertDiscoveredShows(s.ctx, "us", sourceID, []store.DiscoveredShow{
			{Title: "Rated Podcast", FeedURL: "https://example.com/feed.xml", SourceID: sourceID},
		})
		if err != nil {
			t.Fatal(err)
		}
		err = persistShowRating(s.ctx, s.store, "us", "https://example.com/feed.xml", 4.8, 300)
		if err != nil {
			t.Fatal(err)
		}

		rating, count, found := cachedShowRating(s.ctx, s.store, "us", "Rated Podcast", "https://example.com/feed.xml")
		if !found || rating != 4.8 || count != 300 {
			t.Fatalf("expected cached rating (4.8, 300, true), got (%f, %d, %t)", rating, count, found)
		}

		_, _, notFound := cachedShowRating(s.ctx, s.store, "us", "Unknown", "https://example.com/unknown.xml")
		if notFound {
			t.Fatal("expected false for unknown show rating")
		}
	})
}
