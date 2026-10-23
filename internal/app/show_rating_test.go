package app

import (
	"testing"

	"podmutt/internal/discovery"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestShowRatingUsesCachedRatingWithoutNetwork(t *testing.T) {
	librarySession(t, func(s *session) {
		const feedURL = "https://example.test/rated.xml"
		const otherURL = "https://example.test/other.xml"
		directory := &fixtureDirectory{}
		m := browserLibrary(t, s, directory)
		m.browse.active = true
		m.browse.country = "us"
		m.browse.shows = []discovery.Show{
			{Title: "Rated show", FeedURL: feedURL, ExternalID: "123"},
			{Title: "Other show", FeedURL: otherURL, ExternalID: "456"},
		}
		m.Model = m.Model.WithLibrary(ui.Data{Podcasts: []ui.Item{{ID: feedURL, Title: "Rated show"}, {ID: otherURL, Title: "Other show"}}}).Select(ui.Podcasts, feedURL)

		id, err := s.store.SourceID(s.ctx, "apple")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.UpsertDiscoveredShows(s.ctx, "us", id, []store.DiscoveredShow{{Title: "Rated show", FeedURL: feedURL, SourceExternalID: "123", SourceID: id}}); err != nil {
			t.Fatal(err)
		}
		shows, err := s.store.DiscoveredShows(s.ctx, "us", "Rated show", 5)
		if err != nil || len(shows) != 1 {
			t.Fatal(shows, err)
		}
		if err := s.store.UpsertShowRating(s.ctx, shows[0].ID, id, 4.7, 42); err != nil {
			t.Fatal(err)
		}
		if cmd := m.ensureShowRating(); cmd != nil {
			t.Fatal("cached rating triggered network request")
		}
		if m.browse.shows[0].Rating != 4.7 || m.browse.shows[0].ReviewCount != 42 || m.browse.shows[1].Rating != 0 {
			t.Fatal("rating not assigned to selected show", m.browse.shows)
		}
		if cmd := m.ensureShowRating(); cmd != nil {
			t.Fatal("already rated show triggered network request")
		}
		m.Model = m.Model.Select(ui.Podcasts, otherURL)
		if cmd := m.ensureShowRating(); cmd != nil {
			t.Fatal("non-Apple directory triggered rating request")
		}
		if directory.calls != 0 {
			t.Fatal("unexpected directory request", directory.calls)
		}
	})
}
