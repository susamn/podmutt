package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/ui"
)

func TestQueueDeleteAndUndoAcrossLayouts(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		current, pending := m.state.Current.ID, m.state.Queue[0].ID
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.Model = m.Model.WithFocus(ui.Queue)
			for _, key := range []tea.KeyPressMsg{{Code: 'd', Text: "d"}, {Code: tea.KeyDelete}} {
				m.Model = m.Model.Select(ui.Queue, fmt.Sprint(pending))
				finishLibrary(&m, updateLibrary(&m, key))
				if state := syncPlayer(t, &m); len(state.Queue) != 0 || state.Current.ID != current || state.Status != playback.Playing || m.confirming != nil || m.Model.Busy() {
					t.Fatal("single delete was not immediate/pending-only", state)
				}
				if !strings.Contains(m.notice, "u undo") {
					t.Fatal("missing undo guidance", m.notice)
				}
				finishLibrary(&m, libraryKey(&m, 'u')) // Also works on an empty list.
				if state := syncPlayer(t, &m); len(state.Queue) != 1 || state.Queue[0].ID != pending || state.Current.ID != current {
					t.Fatal("undo changed current or lost pending item", state)
				}
			}
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		if m.notice != "Nothing to undo in Up next" || len(syncPlayer(t, &m).Queue) != 1 {
			t.Fatal("used undo affected pending work", m.notice)
		}
		if feeds, err := s.store.Feeds(s.ctx, true); err != nil || len(feeds) != 1 {
			t.Fatal("queue u changed subscriptions", feeds, err)
		}
		p.mu.Lock()
		loads := p.loads
		p.mu.Unlock()
		if loads != 1 {
			t.Fatal("queue edits reloaded audio", loads)
		}
		// Filter editing owns d/u/Delete, not the queue.
		libraryKey(&m, '/')
		libraryKey(&m, 'd')
		libraryKey(&m, 'u')
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyDelete})
		if state := syncPlayer(t, &m); len(state.Queue) != 1 || state.Queue[0].ID != pending {
			t.Fatal("filter input edited the queue", state)
		}
	})
}

func TestUndoPressedWhileRemovalIsInFlightIsNotLost(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue)
		remove := libraryKey(&m, 'd')
		if remove == nil || !m.queueEditing {
			t.Fatal("remove not scheduled")
		}
		if libraryKey(&m, 'd') != nil {
			t.Fatal("duplicate in-flight removal")
		}
		libraryKey(&m, 'D')
		if m.confirming != nil {
			t.Fatal("clear opened over pending removal")
		}
		if libraryKey(&m, 'u') != nil || !m.queueUndoPending {
			t.Fatal("undo intent was not deferred")
		}
		finishLibrary(&m, remove)
		if state := syncPlayer(t, &m); len(state.Queue) != 2 || m.queueEditing || m.queueUndoPending || m.notice != "Restored to Up next" {
			t.Fatal("rapid undo was lost", state, m.notice)
		}
	})
}

func TestStaleRemovalCannotDeleteCurrentOrUndoADifferentItem(t *testing.T) {
	librarySession(t, func(s *session) {
		m, _ := playerLibrary(t, s)
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue)
		first := m.state.Queue[0].ID
		m.Model = m.Model.Select(ui.Queue, fmt.Sprint(first))
		finishLibrary(&m, libraryKey(&m, 'd'))
		syncPlayer(t, &m)
		second := m.state.Queue[0].ID
		m.Model = m.Model.Select(ui.Queue, fmt.Sprint(second))
		remove := libraryKey(&m, 'd')
		if err := s.playback.PlayEpisode(s.ctx, second); err != nil {
			t.Fatal(err)
		}
		libraryKey(&m, 'u')
		finishLibrary(&m, remove)
		if state := syncPlayer(t, &m); len(state.Queue) != 0 || state.Current.ID != second || !strings.Contains(m.notice, "queue changed") {
			t.Fatal("stale remove affected current or undid earlier work", state, m.notice)
		}
		finishLibrary(&m, libraryKey(&m, 'u'))
		if state := syncPlayer(t, &m); len(state.Queue) != 1 || state.Queue[0].ID != first {
			t.Fatal("previous successful undo lost", state)
		}
	})
}
