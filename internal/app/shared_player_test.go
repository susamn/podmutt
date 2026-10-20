package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/ui"
)

func TestSharedPlayerAndQueuePercentKeepInspectionAcrossProgressResizeAndEOF(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		newer, older := m.rows[0], m.rows[1]
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(older.ID))
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		syncPlayer(t, &m)
		m.Model = m.Model.Select(ui.Episodes, fmt.Sprint(newer.ID))
		finishLibrary(&m, libraryKey(&m, 'a'))
		syncPlayer(t, &m)
		libraryKey(&m, 'i')
		if m.detailTarget == nil || m.detailTarget.ID != newer.ID {
			t.Fatal("inspection not pinned")
		}
		entry, _ := p.EntryID(s.ctx)
		p.mu.Lock()
		p.hasSeek, p.seekPosition = true, 1125
		p.mu.Unlock()
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: []byte("1125")}
		deadline := time.Now().Add(2 * time.Second)
		for syncPlayer(t, &m).Current.PositionSeconds != 1125 {
			if time.Now().After(deadline) {
				t.Fatal("current progress missing")
			}
			time.Sleep(time.Millisecond)
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if state := syncPlayer(t, &m); state.Status != playback.Paused || state.Current.ID != older.ID {
			t.Fatal("Space followed inspection", state)
		}
		for _, size := range [][2]int{{140, 44}, {100, 24}, {180, 56}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			view := ansi.Strip(m.View().Content)
			if m.detailTarget.ID != newer.ID || !strings.Contains(view, "75%") || !strings.Contains(view, "Now playing") || !strings.Contains(view, "[Play] [Stop] [Next]") {
				t.Fatal("resize/progress lost inspection, percentage or transport", size, view)
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		syncPlayer(t, &m)
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &entry}
		deadline = time.Now().Add(2 * time.Second)
		for syncPlayer(t, &m).Current.ID != newer.ID {
			if time.Now().After(deadline) {
				t.Fatal("EOF did not advance")
			}
			time.Sleep(time.Millisecond)
		}
		if m.Model.Focus() != ui.Details || m.detailTarget.ID != newer.ID || strings.Contains(ansi.Strip(m.View().Content), "75%") {
			t.Fatal("EOF replaced inspection or retained old percentage")
		}
		if history, err := s.store.Playback(s.ctx, older.ID); err != nil || !history.Completed {
			t.Fatal("history lost", history, err)
		}
	})
}
