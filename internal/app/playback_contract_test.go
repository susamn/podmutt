package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/ui"
)

func TestSpaceControlsCurrentAcrossPanesAndStates(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		libraryKey(&m, 'j')
		other, _ := m.selectedEpisode()
		if other.ID == first.ID {
			t.Fatal("fixture must highlight another episode")
		}
		if err := s.playback.Enqueue(s.ctx, other.ID); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		m.Model = m.Model.Select(ui.Queue, fmt.Sprint(other.ID))
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, panel := range []ui.Panel{ui.Podcasts, ui.Episodes, ui.Queue, ui.Details, ui.PlayerPanel} {
				m.Model = m.Model.WithFocus(panel)
				for _, want := range []playback.Status{playback.Paused, playback.Playing} {
					finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
					state := syncPlayer(t, &m)
					if state.Current.ID != first.ID || state.Status != want || len(state.Queue) != 1 || state.Queue[0].ID != other.ID {
						t.Fatalf("size=%v pane=%v want=%s: %+v", size, panel, want, state)
					}
				}
			}
		}
		p.mu.Lock()
		loads := p.loads
		p.mu.Unlock()
		if loads != 1 {
			t.Fatalf("Space reloaded audio: %d loads", loads)
		}
		finishLibrary(&m, libraryKey(&m, 's'))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if state := syncPlayer(t, &m); state.Current.ID != first.ID || state.Status != playback.Playing {
			t.Fatal("Space did not resume stopped current", state)
		}
	})
}

func TestSpaceWithoutCurrentDoesNotPlaySelectionOrQueue(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		for _, panel := range []ui.Panel{ui.Podcasts, ui.Episodes, ui.Queue, ui.Details, ui.PlayerPanel} {
			m.Model = m.Model.WithFocus(panel)
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
			state := syncPlayer(t, &m)
			if state.Current != nil || state.Status != playback.Stopped || len(state.Queue) != 2 || !strings.Contains(m.notice, "No current episode") {
				t.Fatal("Space started selection/queue or lacked guidance", state, m.notice)
			}
		}
		if p.loads != 0 {
			t.Fatal("Space loaded audio without current")
		}
	})
}

func TestEnterPlaysWithoutTogglingAndDetailsReturnToOrigin(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		first, _ := m.selectedEpisode()
		for range 2 {
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
			if state := syncPlayer(t, &m); state.Current.ID != first.ID || state.Status != playback.Playing || m.Model.Focus() != ui.Episodes {
				t.Fatal("Enter toggled or left episodes", state)
			}
		}
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, panel := range []ui.Panel{ui.Episodes, ui.Queue} {
				m.Model = m.Model.WithFocus(panel)
				selected, _ := m.Model.Selected(panel)
				libraryKey(&m, 'i')
				if m.Model.Focus() != ui.Details || m.detailTarget == nil {
					t.Fatal("i did not open details", panel)
				}
				updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
				after, _ := m.Model.Selected(panel)
				if m.Model.Focus() != panel || after.ID != selected.ID || m.detailTarget != nil {
					t.Fatal("Esc lost origin or selection", panel)
				}
			}
		}
		m.Model = m.Model.WithFocus(ui.Podcasts)
		libraryKey(&m, 'i')
		if m.Model.Focus() != ui.Podcasts {
			t.Fatal("i opened unrelated details")
		}
		m.Model = m.Model.ClearFilter(ui.Episodes).WithFocus(ui.Episodes)
		libraryKey(&m, '/')
		updateLibrary(&m, tea.PasteMsg{Content: "no matching episode"})
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter})
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if !strings.Contains(m.notice, "Select an episode") || syncPlayer(t, &m).Current.ID != first.ID {
			t.Fatal("empty list played unrelated current", m.notice)
		}
	})
}

func TestPlaybackKeysStayInsideInputAndOverlays(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		for _, opener := range []rune{'/', 'f', 'U', 'N', '?', 'e'} {
			m.Model = m.Model.WithFocus(ui.Episodes)
			libraryKey(&m, opener)
			finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}))
			libraryKey(&m, 'i')
			// Enter belongs to each editor/overlay, not episode playback.
			if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
				updateLibrary(&m, cmd()) // Do not execute live theme-reload subscriptions.
			}
			updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
			if state := syncPlayer(t, &m); state.Current != nil || m.Model.Focus() == ui.Details {
				t.Fatal("input/overlay dispatched playback or details", string(opener), state)
			}
		}
		if p.loads != 0 {
			t.Fatal("input loaded audio")
		}
	})
}
