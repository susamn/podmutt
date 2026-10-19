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
			for _, arrows := range []bool{false, true} {
				open := func() tea.Cmd { return libraryKey(&m, 'l') }
				back := func() tea.Cmd { return libraryKey(&m, 'h') }
				if arrows {
					open = func() tea.Cmd { return updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyRight}) }
					back = func() tea.Cmd { return updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyLeft}) }
				}
				m.Model = m.Model.WithFocus(ui.Podcasts).Select(ui.Podcasts, fmt.Sprint(m.snapshot.Feeds[0].ID))
				show, _ := m.Model.Selected(ui.Podcasts)
				finishLibrary(&m, open())
				if m.Model.Focus() != ui.Episodes || m.libraryFeedID() == 0 || len(m.rows) == 0 {
					t.Fatal("open did not descend into show")
				}
				selected, _ := m.selectedEpisode()
				finishLibrary(&m, open())
				if m.Model.Focus() != ui.Details || m.detailTarget == nil || m.detailTarget.ID != selected.ID {
					t.Fatal("open did not inspect episode")
				}
				finishLibrary(&m, back())
				item, _ := m.Model.Selected(ui.Episodes)
				if m.Model.Focus() != ui.Episodes || item.ID != fmt.Sprint(selected.ID) {
					t.Fatal("back lost episode origin")
				}
				finishLibrary(&m, back())
				after, _ := m.Model.Selected(ui.Podcasts)
				if m.Model.Focus() != ui.Podcasts || after.ID != show.ID {
					t.Fatal("back lost show origin")
				}
				if state := syncPlayer(t, &m); state.Current != nil {
					t.Fatal("navigation started audio", state)
				}
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
		libraryKey(&m, 'd')
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
		finishLibrary(&m, libraryKey(&m, 'l'))
		finishLibrary(&m, libraryKey(&m, 'h'))
		after, _ := m.Model.Selected(ui.Queue)
		if m.Model.Focus() != ui.Queue || after.ID != item.ID || syncPlayer(t, &m).Current != nil {
			t.Fatal("queue inspection changed focus, identity, or playback")
		}
	})
}
