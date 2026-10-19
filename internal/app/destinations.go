package app

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/library"
	"podmutt/internal/ui"
)

const (
	newDestination      = "library:new"
	continueDestination = "library:continue"
	allDestination      = "library:all"
)

var libraryDestinations = []struct {
	id   string
	view library.View
}{
	{newDestination, library.NewView},
	{continueDestination, library.ContinueView},
	{allDestination, library.AllView},
}

func (m libraryShell) libraryView() library.View {
	for _, destination := range libraryDestinations {
		if m.libraryID == destination.id {
			return destination.view
		}
	}
	return library.ShowView
}

func (m libraryShell) libraryFeedID() int64 {
	id, _ := strconv.ParseInt(m.libraryID, 10, 64)
	return id
}

// Synchronize the canonical destination with the sidebar, including fallback
// after unsubscribe/filter changes. Pending add/restore selections win until
// their cached rows arrive; search results have their own explicit scope.
func (m *libraryShell) syncLibrarySelection() tea.Cmd {
	if m.browse.active || m.browse.localResults || m.wantLibraryID != "" {
		return nil
	}
	item, _ := m.Model.Selected(ui.Podcasts)
	if item.ID == m.libraryID {
		return nil
	}
	m.libraryID = item.ID
	m.snapshot.Show = nil
	m.Model = m.Model.Reset(ui.Episodes)
	m.renderLibrary()
	return m.reload()
}
