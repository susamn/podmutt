package app

import (
	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/ui"
	"testing"
)

func TestEmptyLibraryRefreshIsNotPerpetuallyBusy(t *testing.T) {
	librarySession(t, func(s *session) {
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		if m.notice != "Cached library" {
			t.Fatal("empty library claims an active refresh")
		}
		// An empty profile starts in Discover; the library remains reachable
		// without waiting for or running any online command in this unit test.
		libraryKey(&m, 'b')
		libraryKey(&m, 'R')
		if m.notice != "No subscribed feeds to refresh" {
			t.Fatal(m.notice)
		}
		libraryKey(&m, 'u')
		if m.mutating {
			t.Fatal("empty library dispatched subscription")
		}
		libraryKey(&m, 'c')
		if m.mutating {
			t.Fatal("empty library dispatched completion")
		}
	})
}

func TestNavigationAndInputReuseCachedRows(t *testing.T) {
	librarySession(t, func(s *session) {
		seededLibrary(t, s)
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		rows := &m.rows[0]
		m.Model = m.Model.WithFocus(ui.Episodes)
		libraryKey(&m, 'j')
		libraryKey(&m, 'N')
		updateLibrary(&m, tea.PasteMsg{Content: "https://example.test/feed"})
		if &m.rows[0] != rows {
			t.Fatal("navigation/input rebuilt and resorted the cached library")
		}
	})
}
