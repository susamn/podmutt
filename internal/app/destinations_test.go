package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/playback"
	"podmutt/internal/ui"
)

func TestExplicitDestinationsTrackSidebarWithoutModeToggle(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		showID := m.snapshot.Feeds[0].ID
		if err := s.store.SetCompleted(s.ctx, m.rows[0].ID, true); err != nil {
			t.Fatal(err)
		}
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			openLibraryDestination(&m, newDestination)
			if len(m.rows) != 1 {
				t.Fatal("New episodes includes completed history")
			}
			openLibraryDestination(&m, allDestination)
			if len(m.rows) != 2 {
				t.Fatal("All subscriptions omitted completed history")
			}
			m.Model = m.Model.WithFocus(ui.Podcasts)
			finishLibrary(&m, libraryKey(&m, 'j')) // All subscriptions -> Alpha show
			if m.libraryFeedID() != showID || len(m.rows) != 2 {
				t.Fatal("highlighted show did not immediately scope episodes", m.libraryID, m.rows)
			}
			item, _ := m.Model.Selected(ui.Podcasts)
			if item.ID != m.libraryID || item.Destination {
				t.Fatal("highlight and episode scope disagree", item)
			}
			libraryKey(&m, 'v')
			if m.libraryID != fmt.Sprint(showID) || len(m.rows) != 2 {
				t.Fatal("obsolete v key changed scope")
			}
		}
		updateLibrary(&m, tea.WindowSizeMsg{Width: 140, Height: 40})
		if !strings.Contains(m.View().Content, "Show: Alpha show") || p.loads != 0 || syncPlayer(t, &m).Current != nil {
			t.Fatal("scope label missing or browsing autoplayed")
		}
	})
}

func TestLibrarySectionLabelsDoNotBecomeActions(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 160, Height: 44})
		openLibraryDestination(&m, allDestination)
		items := m.Model.Visible(ui.Podcasts)
		if items[2].Title != "All episodes" || len(items) != len(libraryDestinations)+1 {
			t.Fatal("short view label or selectable rows changed", items)
		}
		if view := m.View().Content; !strings.Contains(view, "All subscribed episodes") || !strings.Contains(view, "Subscriptions") {
			t.Fatal("scope or section label missing", view)
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		libraryKey(&m, 'u')
		finishLibrary(&m, libraryKey(&m, 'j'))
		if m.libraryFeedID() != m.snapshot.Feeds[0].ID || len(m.rows) != 2 || p.loads != 0 {
			t.Fatal("heading became an action or disrupted show navigation")
		}
	})
}

func TestContinueListeningResumesUnsubscribedHistoryAndRemovesFinished(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		preview, err := s.store.Episodes(s.ctx, m.snapshot.Feeds[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.Checkpoint(s.ctx, preview[0].ID, 73); err != nil {
			t.Fatal(err)
		}
		openLibraryDestination(&m, continueDestination)
		if len(m.rows) != 1 || m.rows[0].ID != preview[0].ID || p.loads != 0 {
			t.Fatal("Continue omitted unsubscribed resume or autoplayed", m.rows)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		state := syncPlayer(t, &m)
		if state.Current.ID != preview[0].ID || state.Current.PositionSeconds != 73 || state.Status != playback.Playing {
			t.Fatal("Enter did not resume saved history", state)
		}
		feeds, err := s.store.Feeds(s.ctx, true)
		if err != nil || len(feeds) != 1 {
			t.Fatal("resume subscribed preview", feeds, err)
		}
		finishLibrary(&m, libraryKey(&m, 'c'))
		if len(m.rows) != 0 || !strings.Contains(m.View().Content, "Nothing to resume") {
			t.Fatal("finished episode remained in Continue")
		}
	})
}

func TestDestinationsSurviveDiscoverySearchAndStaleReads(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		openLibraryDestination(&m, allDestination)
		m.browse.directory = &fixtureDirectory{}
		finishLibrary(&m, libraryKey(&m, 'b'))
		finishLibrary(&m, libraryKey(&m, 'b'))
		if item, _ := m.Model.Selected(ui.Podcasts); item.ID != allDestination || m.libraryID != allDestination || len(m.rows) != 2 {
			t.Fatal("discovery roundtrip lost destination", item, m.libraryID)
		}
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		for _, item := range m.Model.Visible(ui.Podcasts) {
			if item.Destination {
				t.Fatal("navigation destinations leaked into search results")
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		if item, _ := m.Model.Selected(ui.Podcasts); item.ID != allDestination || m.libraryID != allDestination {
			t.Fatal("search roundtrip lost destination", item, m.libraryID)
		}
		m.Model = m.Model.WithFocus(ui.Podcasts).Select(ui.Podcasts, continueDestination)
		pending := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		openLibraryDestination(&m, fmt.Sprint(m.snapshot.Feeds[0].ID))
		finishLibrary(&m, pending)
		if m.libraryFeedID() == 0 || len(m.rows) != 2 {
			t.Fatal("stale Continue read replaced selected show", m.libraryID, m.rows)
		}
	})
}

func TestNoSubscriptionsWithUnfinishedHistoryStartsInContinueStopped(t *testing.T) {
	librarySession(t, func(s *session) {
		_, p := playerLibrary(t, s)
		feeds, err := s.store.Feeds(s.ctx, true)
		if err != nil {
			t.Fatal(err)
		}
		episodes, err := s.store.Episodes(s.ctx, feeds[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.Checkpoint(s.ctx, episodes[0].ID, 42); err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetSubscribed(s.ctx, feeds[0].ID, false); err != nil {
			t.Fatal(err)
		}
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil || m.browse.active || m.libraryID != continueDestination || len(m.rows) != 1 {
			t.Fatal("unfinished preview history hidden behind Discover", err)
		}
		if state := syncPlayer(t, &m); state.Status != playback.Stopped || p.loads != 0 {
			t.Fatal("Continue startup autoplayed", state)
		}
	})
}

func TestUnsubscribeAndEmptyFilterKeepDestinationConsistent(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		openLibraryDestination(&m, fmt.Sprint(m.snapshot.Feeds[0].ID))
		m.Model = m.Model.WithFocus(ui.Podcasts)
		finishLibrary(&m, libraryKey(&m, 'u'))
		item, _ := m.Model.Selected(ui.Podcasts)
		if m.wantLibraryID != "" || m.libraryID != item.ID || !item.Destination {
			t.Fatal("unsubscribe left stale selected show", m.libraryID, item)
		}
		libraryKey(&m, '/')
		finishLibrary(&m, updateLibrary(&m, tea.PasteMsg{Content: "no match"}))
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if len(m.rows) != 0 {
			t.Fatal("empty sidebar filter retained unrelated episodes")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		item, _ = m.Model.Selected(ui.Podcasts)
		if m.libraryID != item.ID || m.libraryID == "" {
			t.Fatal("clearing filter did not restore valid destination")
		}
	})
}
