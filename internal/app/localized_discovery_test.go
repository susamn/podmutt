package app

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestLocalizedDiscoveryLocalCacheAndRatings(t *testing.T) {
	librarySession(t, func(s *session) {
		s.config.Country = "de"
		refresher := feed.New(s.store, nil, 0)
		m, err := newLibrary(s, refresher)
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})

		// Pre-populate discovery catalog in SQLite
		appleID, err := s.store.SourceID(s.ctx, "apple")
		if err != nil {
			t.Fatal(err)
		}
		show := store.DiscoveredShow{
			Title:            "Lage der Nation",
			FeedURL:          "https://feeds.lagedernation.org/ldn",
			Description:      "German political podcast",
			Country:          "de",
			SourceID:         appleID,
			SourceExternalID: "1157396112",
		}
		if err := s.store.UpsertDiscoveredShows(s.ctx, "de", appleID, []store.DiscoveredShow{show}); err != nil {
			t.Fatal(err)
		}
		shows, err := s.store.DiscoveredShows(s.ctx, "de", "Lage", 10)
		if err != nil || len(shows) != 1 {
			t.Fatal(shows, err)
		}
		if err := s.store.UpsertShowRating(s.ctx, shows[0].ID, appleID, 4.8, 12500); err != nil {
			t.Fatal(err)
		}

		// Switch to Discover mode: loads local cache with 0 network calls
		finishLibrary(&m, m.switchBrowse())
		finishLibrary(&m, m.searchDirectory(""))

		view := m.View().Content
		if !strings.Contains(view, "Lage der Nation") {
			t.Fatalf("expected cached German show in discovery view: %s", view)
		}

		// Search for show in local discovery database
		finishLibrary(&m, m.searchDirectory("Lage"))
		view = m.View().Content
		if !strings.Contains(view, "Lage der Nation") {
			t.Fatalf("expected query to find cached show: %s", view)
		}

		// Select show and verify customer star rating in Details
		m.Model = m.Model.Select(ui.Podcasts, "https://feeds.lagedernation.org/ldn")
		m.renderDetails()
		view = m.View().Content
		if !strings.Contains(view, "★ 4.8") {
			t.Fatalf("expected customer rating ★ 4.8 in details: %s", view)
		}
	})
}

func TestSettingsInteractiveCountrySwitch(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})

		// Open Settings ('e')
		updateLibrary(&m, tea.KeyPressMsg{Code: 'e', Text: "e"})
		if !m.Model.SettingsOpen() {
			t.Fatal("settings not open")
		}

		view := m.View().Content
		if !strings.Contains(view, "Country:") || !strings.Contains(view, "Left/Right to change") {
			t.Fatalf("expected Country row in settings: %s", view)
		}

		// Cycle country with 'right' key
		cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyRight})
		if cmd != nil {
			msg := cmd()
			updateLibrary(&m, msg)
		}
		if s.config.Country == "" || s.config.Country == "us" {
			t.Fatalf("expected country to change from default us, got %q", s.config.Country)
		}

		// Verify Apple directory updated its country
		if apple, ok := m.browse.directory.(*discovery.Apple); ok {
			if apple.Country() != s.config.Country {
				t.Fatalf("expected directory country %q, got %q", s.config.Country, apple.Country())
			}
		}
	})
}

type mockShowDirectory struct {
	topCalls    int
	searchCalls int
	shows       []discovery.Show
}

func (d *mockShowDirectory) Top(ctx context.Context) ([]discovery.Show, error) {
	d.topCalls++
	return d.shows, nil
}

func (d *mockShowDirectory) Search(ctx context.Context, query string) ([]discovery.Show, error) {
	d.searchCalls++
	return d.shows, nil
}
