package app

import (
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestVisibleCurrentIsProtectedFromPendingEditsAcrossLayouts(t *testing.T) {
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
			t.Fatal("fixture needs two pending")
		}
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, status := range []playback.Status{playback.Paused, playback.Stopped, playback.Failed, playback.Playing} {
				state := m.state
				state.Status = status
				updateLibrary(&m, playbackUpdate{state: state, ok: true})
				rows := m.Model.Visible(ui.Queue)
				current := 0
				if len(rows) != 3 {
					t.Fatal("current not projected exactly once", rows)
				}
				for i, row := range rows {
					if row.Current {
						current++
						if row.ID != fmt.Sprint(first.ID) {
							t.Fatal("wrong current", row)
						}
					}
					if row.Index != i+1 {
						t.Fatal("bad numbering", row)
					}
				}
				if current != 1 {
					t.Fatal("current duplicated")
				}
				m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(first.ID))
				if libraryKey(&m, 'd') != nil || m.queueEditing || m.confirming == nil {
					t.Fatal("current removal must await confirmation")
				}
				libraryKey(&m, 'n')
			}
		}
		syncPlayer(t, &m)
		before := append([]store.Episode(nil), m.state.Queue...)
		pending := before[0].ID
		m.Model = m.Model.WithFocus(ui.Queue).Select(ui.Queue, fmt.Sprint(pending))
		libraryKey(&m, 'J')
		libraryKey(&m, 'K')
		libraryKey(&m, 'o')
		syncPlayer(t, &m)
		if !slices.EqualFunc(before, m.state.Queue, func(a, b store.Episode) bool { return a.ID == b.ID }) {
			t.Fatal("view operations mutated actor queue")
		}
		finishLibrary(&m, libraryKey(&m, 'd'))
		finishLibrary(&m, libraryKey(&m, 'y'))
		if state := syncPlayer(t, &m); len(state.Queue) != 1 || state.Current.ID != first.ID || state.Status != playback.Playing {
			t.Fatal("pending deletion changed transport", state)
		}
		libraryKey(&m, 'D')
		finishLibrary(&m, libraryKey(&m, 'y'))
		if state := syncPlayer(t, &m); len(state.Queue) != 0 || state.Current.ID != first.ID || len(m.Model.Visible(ui.Queue)) != 1 {
			t.Fatal("clear removed current", state)
		}
		p.mu.Lock()
		loads := p.loads
		p.mu.Unlock()
		if loads != 1 {
			t.Fatal("edits reloaded audio", loads)
		}
	})
}
