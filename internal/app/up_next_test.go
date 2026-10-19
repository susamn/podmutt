package app

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestUpNextEditsCannotTargetCurrentAcrossLayouts(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		if err := s.store.UpsertEpisodes(s.ctx, m.snapshot.Feeds[0].ID, []store.Episode{{GUID: "third", Title: "Third pending", AudioURL: "https://example.test/third.mp3"}}); err != nil {
			t.Fatal(err)
		}
		finishLibrary(&m, m.reload())
		first, _ := m.selectedEpisode()
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		finishLibrary(&m, libraryKey(&m, 'A'))
		syncPlayer(t, &m)
		if len(m.state.Queue) != 2 {
			t.Fatal("fixture needs two pending episodes")
		}
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, status := range []playback.Status{playback.Paused, playback.Stopped, playback.Failed, playback.Playing} {
				state := m.state
				state.Status = status
				updateLibrary(&m, playbackUpdate{state: state, ok: true})
				rows := m.Model.Visible(ui.Queue)
				if len(rows) != 2 {
					t.Fatal("transport changed pending rows", status, rows)
				}
				for i, row := range rows {
					if row.ID == fmt.Sprint(first.ID) || row.Index != i+1 {
						t.Fatal("current is editable or pending numbering wrong", status, row)
					}
				}
			}
		}
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.Queue)
		selected, _ := m.Model.Selected(ui.Queue)
		finishLibrary(&m, libraryKey(&m, 'J'))
		syncPlayer(t, &m)
		if after, _ := m.Model.Selected(ui.Queue); after.ID != selected.ID || fmt.Sprint(m.state.Queue[1].ID) != selected.ID {
			t.Fatal("reorder lost pending identity", after, m.state.Queue)
		}
		libraryKey(&m, 'd')
		if m.confirming == nil || m.confirming.ids[0] == first.ID {
			t.Fatal("remove did not target a pending entry")
		}
		finishLibrary(&m, libraryKey(&m, 'y'))
		if state := syncPlayer(t, &m); len(state.Queue) != 1 || state.Current.ID != first.ID || state.Status != playback.Playing {
			t.Fatal("remove affected transport", state)
		}
		libraryKey(&m, 'D')
		finishLibrary(&m, libraryKey(&m, 'y'))
		if state := syncPlayer(t, &m); len(state.Queue) != 0 || state.Current.ID != first.ID || state.Status != playback.Playing {
			t.Fatal("clear affected transport", state)
		}
		p.mu.Lock()
		loads := p.loads
		p.mu.Unlock()
		if loads != 1 {
			t.Fatal("queue edits reloaded current audio", loads)
		}
	})
}
