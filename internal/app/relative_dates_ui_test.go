package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/discovery"
	"podmutt/internal/feed"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestEpisodesAndDetailsRenderHumanReadableRelativeDates(t *testing.T) {
	loc := time.FixedZone("EST", -5*3600)
	fixedNow := time.Date(2026, 10, 22, 12, 0, 0, 0, loc)

	oldTime := currentTime
	currentTime = func() time.Time { return fixedNow }
	defer func() { currentTime = oldTime }()

	librarySession(t, func(s *session) {
		feedID, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: "https://example.test/dates", Title: "Dates Show", Subscribed: true})
		if err != nil {
			t.Fatal(err)
		}

		episodes := []store.Episode{
			{GUID: "today", Title: "Today Ep", FeedID: feedID, AudioURL: "https://example.test/1.mp3", PublishedAt: time.Date(2026, 10, 22, 8, 0, 0, 0, loc), DurationSeconds: 1200},
			{GUID: "yesterday", Title: "Yesterday Ep", FeedID: feedID, AudioURL: "https://example.test/2.mp3", PublishedAt: time.Date(2026, 10, 21, 20, 0, 0, 0, loc), DurationSeconds: 1200},
			{GUID: "2days", Title: "Two Days Ep", FeedID: feedID, AudioURL: "https://example.test/3.mp3", PublishedAt: time.Date(2026, 10, 20, 10, 0, 0, 0, loc), DurationSeconds: 1200},
			{GUID: "7days", Title: "Week Back Ep", FeedID: feedID, AudioURL: "https://example.test/4.mp3", PublishedAt: time.Date(2026, 10, 15, 12, 0, 0, 0, loc), DurationSeconds: 1200},
			{GUID: "8days", Title: "Older Ep", FeedID: feedID, AudioURL: "https://example.test/5.mp3", PublishedAt: time.Date(2026, 10, 14, 12, 0, 0, 0, loc), DurationSeconds: 1200},
		}
		if err := s.store.UpsertEpisodes(s.ctx, feedID, episodes); err != nil {
			t.Fatal(err)
		}

		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		// Wide terminal (160x40): episodeWidth >= 70 so date column is visible
		updateLibrary(&m, tea.WindowSizeMsg{Width: 160, Height: 40})
		m.Model = m.Model.WithFocus(ui.Episodes)
		m.renderLibrary()

		content := ansi.Strip(m.View().Content)

		// Check Episodes rows
		for _, want := range []string{"Today", "Yesterday", "2 days back", "A week back", "2026-10-14"} {
			if !strings.Contains(content, want) {
				t.Fatalf("Episodes pane missing date %q:\n%s", want, content)
			}
		}

		// Check Details pane for selected episode
		// Initially selected is first episode ("Today Ep")
		m.renderDetails()
		detailsView := ansi.Strip(m.View().Content)
		if !strings.Contains(detailsView, "Today · 20:00 · Not started") {
			t.Fatalf("Details missing 'Today' relative date:\n%s", detailsView)
		}

		// Select "Week Back Ep" (index 3)
		savedEpisodes, _ := s.store.Episodes(s.ctx, feedID)
		for _, ep := range savedEpisodes {
			if ep.GUID == "7days" {
				m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(ep.ID))
				break
			}
		}
		m.renderDetails()
		detailsView = ansi.Strip(m.View().Content)
		if !strings.Contains(detailsView, "A week back · 20:00 · Not started") {
			t.Fatalf("Details missing 'A week back' date:\n%s", detailsView)
		}

		// Select "Older Ep" (index 4)
		for _, ep := range savedEpisodes {
			if ep.GUID == "8days" {
				m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(ep.ID))
				break
			}
		}
		m.renderDetails()
		detailsView = ansi.Strip(m.View().Content)
		if !strings.Contains(detailsView, "2026-10-14 · 20:00 · Not started") {
			t.Fatalf("Details missing '2026-10-14' fallback date:\n%s", detailsView)
		}
	})
}

func TestDiscoverPreviewRendersRelativeDates(t *testing.T) {
	loc := time.UTC
	fixedNow := time.Date(2026, 10, 22, 12, 0, 0, 0, loc)

	oldTime := currentTime
	currentTime = func() time.Time { return fixedNow }
	defer func() { currentTime = oldTime }()

	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		m.browse.active = true
		m.browse.shows = []discovery.Show{{Title: "Remote Show", FeedURL: "https://example.test/feed"}}
		m.browse.preview = &feed.Resolved{
			URL: "https://example.test/feed",
			Fetched: feed.Fetched{
				Document: feed.Document{
					Title: "Remote Show",
					Episodes: []store.Episode{
						{GUID: "ep1", Title: "Fresh Ep", PublishedAt: time.Date(2026, 10, 22, 9, 0, 0, 0, loc), DurationSeconds: 600},
						{GUID: "ep2", Title: "Yesterday Ep", PublishedAt: time.Date(2026, 10, 21, 9, 0, 0, 0, loc), DurationSeconds: 600},
						{GUID: "ep3", Title: "Old Ep", PublishedAt: time.Date(2026, 10, 10, 9, 0, 0, 0, loc), DurationSeconds: 600},
					},
				},
			},
		}

		// Wide terminal (160x40): episodeWidth >= 70 so date column is visible
		updateLibrary(&m, tea.WindowSizeMsg{Width: 160, Height: 40})
		m.Model = m.Model.WithFocus(ui.Episodes)
		m.renderLibrary()

		content := ansi.Strip(m.View().Content)
		for _, want := range []string{"Today", "Yesterday", "2026-10-10"} {
			if !strings.Contains(content, want) {
				t.Fatalf("Discover preview missing date %q:\n%s", want, content)
			}
		}
	})
}

func TestMidnightTickUpdatesRelativeDatesWithoutNetworkOrLostSelection(t *testing.T) {
	loc := time.FixedZone("EST", -5*3600)
	// Day 1: 2026-10-22 23:59:50
	day1 := time.Date(2026, 10, 22, 23, 59, 50, 0, loc)
	day2 := time.Date(2026, 10, 23, 0, 0, 5, 0, loc)

	currentMockTime := day1
	oldTime := currentTime
	currentTime = func() time.Time { return currentMockTime }
	defer func() { currentTime = oldTime }()

	librarySession(t, func(s *session) {
		feedID, err := s.store.UpsertFeed(s.ctx, store.Feed{URL: "https://example.test/midnight", Title: "Midnight Show", Subscribed: true})
		if err != nil {
			t.Fatal(err)
		}

		episodes := []store.Episode{
			{GUID: "ep-today", Title: "Target Episode", FeedID: feedID, AudioURL: "https://example.test/today.mp3", PublishedAt: time.Date(2026, 10, 22, 10, 0, 0, 0, loc), DurationSeconds: 900},
		}
		if err := s.store.UpsertEpisodes(s.ctx, feedID, episodes); err != nil {
			t.Fatal(err)
		}

		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})
		m.Model = m.Model.WithFocus(ui.Episodes)
		m.renderLibrary()

		// Before midnight: episode is "Today"
		content := ansi.Strip(m.View().Content)
		if !strings.Contains(content, "Today") {
			t.Fatalf("expected 'Today' before midnight:\n%s", content)
		}

		// Advance clock to after midnight (Day 2)
		currentMockTime = day2

		// Dispatch midnightTick
		cmd := updateLibrary(&m, midnightTick{})
		if cmd == nil {
			t.Fatal("midnightTick did not rearm next midnight timer")
		}

		// After midnight: episode must now be "Yesterday"
		contentAfter := ansi.Strip(m.View().Content)
		if strings.Contains(contentAfter, "Today") {
			t.Fatalf("still contains 'Today' after midnight:\n%s", contentAfter)
		}
		if !strings.Contains(contentAfter, "Yesterday") {
			t.Fatalf("expected 'Yesterday' after midnight:\n%s", contentAfter)
		}

		// Details also updated
		detailsView := ansi.Strip(m.View().Content)
		if !strings.Contains(detailsView, "Yesterday · 15:00 · Not started") {
			t.Fatalf("Details missing 'Yesterday' after midnight:\n%s", detailsView)
		}

		// Selection still targets the episode
		selected, ok := m.selectedEpisode()
		if !ok || selected.Title != "Target Episode" {
			t.Fatalf("selection lost after midnight tick: %+v", selected)
		}
	})
}
