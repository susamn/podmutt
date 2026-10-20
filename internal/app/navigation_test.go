package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/ui"
)

func TestOpenAndBackNavigateWithoutStartingPlayback(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.Model = m.Model.WithFocus(ui.Podcasts).Select(ui.Podcasts, fmt.Sprint(m.snapshot.Feeds[0].ID))
			show, _ := m.Model.Selected(ui.Podcasts)
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
			if m.Model.Focus() != ui.Episodes || m.libraryFeedID() == 0 || len(m.rows) == 0 {
				t.Fatal("Enter did not descend into show")
			}
			selected, _ := m.selectedEpisode()
			finishLibrary(&m, libraryKey(&m, 'i'))
			if m.Model.Focus() != ui.Details || m.detailTarget == nil || m.detailTarget.ID != selected.ID {
				t.Fatal("i did not inspect episode")
			}
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
			item, _ := m.Model.Selected(ui.Episodes)
			if m.Model.Focus() != ui.Episodes || item.ID != fmt.Sprint(selected.ID) {
				t.Fatal("Esc lost episode origin")
			}
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
			after, _ := m.Model.Selected(ui.Podcasts)
			if m.Model.Focus() != ui.Podcasts || after.ID != show.ID {
				t.Fatal("Esc lost show origin")
			}
			if state := syncPlayer(t, &m); state.Current != nil {
				t.Fatal("navigation started audio", state)
			}
			// h/l scroll columns without changing panels
			libraryKey(&m, 'l')
			if m.Model.Focus() != ui.Podcasts {
				t.Fatal("l switched panels instead of scrolling")
			}
			libraryKey(&m, 'h')
			if m.Model.Focus() != ui.Podcasts {
				t.Fatal("h switched panels instead of scrolling")
			}
		}
		if p.loads != 0 {
			t.Fatal("navigation loaded audio")
		}
	})
}

func TestNavigationStaysInsideEditorsAndOverlays(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		for _, opener := range []rune{'/', 'f', 'U', 'N', '?', 'e'} {
			m.Model = m.Model.WithFocus(ui.Episodes)
			libraryKey(&m, opener)
			libraryKey(&m, 'h')
			libraryKey(&m, 'l')
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyLeft})
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyRight})
			if m.Model.Focus() != ui.Episodes {
				t.Fatal("navigation escaped editor/overlay", string(opener))
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		}
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue)
		libraryKey(&m, 'D')
		libraryKey(&m, 'h')
		libraryKey(&m, 'l')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyLeft})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyRight})
		if m.confirming == nil || m.Model.Focus() != ui.Queue || len(syncPlayer(t, &m).Queue) != 2 {
			t.Fatal("navigation escaped confirmation")
		}
	})
}

func TestQueueInspectionReturnsToQueueWithoutChangingPlayback(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue)
		item, _ := m.Model.Selected(ui.Queue)
		finishLibrary(&m, libraryKey(&m, 'i'))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape}))
		after, _ := m.Model.Selected(ui.Queue)
		if m.Model.Focus() != ui.Queue || after.ID != item.ID || syncPlayer(t, &m).Current != nil {
			t.Fatal("queue inspection changed focus, identity, or playback")
		}
	})
}
