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

func TestLibraryHasOnlyAllAndSubscriptionsWithImmediateScopes(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		showID := m.snapshot.Feeds[0].ID
		for _, size := range [][2]int{{160, 44}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			openLibraryDestination(&m, allDestination)
			items := m.Model.Visible(ui.Podcasts)
			if len(items) != 2 || items[0].ID != allDestination || items[0].Title != "All episodes" || !items[0].Destination || items[1].Destination {
				t.Fatal("extra views or selectable heading", items)
			}
			m.Model = m.Model.WithFocus(ui.Podcasts)
			finishLibrary(&m, libraryKey(&m, 'j'))
			if m.libraryFeedID() != showID || len(m.rows) != 2 {
				t.Fatal("highlight did not scope Episodes", m.libraryID)
			}
			finishLibrary(&m, libraryKey(&m, 'k'))
			if m.libraryID != allDestination || len(m.rows) != 2 {
				t.Fatal("All did not restore immediately")
			}
		}
		if p.loads != 0 || syncPlayer(t, &m).Current != nil {
			t.Fatal("browsing started audio")
		}
	})
}

func TestAllKeepsProgressCompletionAndUnsubscribedSavedWork(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		preview, err := s.store.Episodes(s.ctx, m.snapshot.Feeds[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.Checkpoint(s.ctx, preview[0].ID, 73); err != nil {
			t.Fatal(err)
		}
		openLibraryDestination(&m, allDestination)
		if len(m.rows) != 3 || p.loads != 0 {
			t.Fatal("saved unsubscribed work hidden", m.rows)
		}
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(preview[0].ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		state := syncPlayer(t, &m)
		if state.Current.ID != preview[0].ID || state.Current.PositionSeconds != 73 || state.Status != playback.Playing {
			t.Fatal(state)
		}
		finishLibrary(&m, libraryKey(&m, 'c'))
		if selected, ok := m.selectedEpisode(); !ok || !selected.Completed || len(m.rows) != 3 {
			t.Fatal("completion hid selected saved work", m.rows)
		}
		feeds, err := s.store.Feeds(s.ctx, true)
		if err != nil || len(feeds) != 1 {
			t.Fatal("playback subscribed preview", feeds, err)
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
		if item, _ := m.Model.Selected(ui.Podcasts); item.ID != allDestination || len(m.rows) != 2 {
			t.Fatal("discovery lost scope", item)
		}
		libraryKey(&m, 'f')
		updateLibrary(&m, tea.PasteMsg{Content: "Alpha"})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		for _, item := range m.Model.Visible(ui.Podcasts) {
			if item.Destination {
				t.Fatal("destination leaked into search")
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		if item, _ := m.Model.Selected(ui.Podcasts); item.ID != allDestination || m.libraryID != allDestination {
			t.Fatal("search lost scope", item)
		}
		pending := m.reload()
		openLibraryDestination(&m, fmt.Sprint(m.snapshot.Feeds[0].ID))
		finishLibrary(&m, pending)
		if m.libraryFeedID() == 0 || len(m.rows) != 2 {
			t.Fatal("stale read replaced show")
		}
	})
}

func TestUnsubscribedSavedHistoryOpensAllWithoutStartingConstructor(t *testing.T) {
	librarySession(t, func(s *session) {
		old, p := playerLibrary(t, s)
		id := old.rows[0].ID
		if err := s.store.Checkpoint(s.ctx, id, 42); err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetSubscribed(s.ctx, old.snapshot.Feeds[0].ID, false); err != nil {
			t.Fatal(err)
		}
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil || m.browse.active || m.libraryID != allDestination || len(m.rows) != 1 {
			t.Fatal("saved history hidden", m.rows, err)
		}
		if p.loads != 0 {
			t.Fatal("constructor performed startup command")
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
			t.Fatal("unsubscribe left stale scope", item)
		}
		libraryKey(&m, '/')
		finishLibrary(&m, updateLibrary(&m, tea.PasteMsg{Content: "no match"}))
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if len(m.rows) != 0 {
			t.Fatal("empty filter kept unrelated rows")
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		item, _ = m.Model.Selected(ui.Podcasts)
		if m.libraryID != item.ID || !strings.HasPrefix(m.libraryID, "library:") {
			t.Fatal("clear did not restore All")
		}
	})
}
