package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/playback"
	"podmutt/internal/player"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func TestCompactTransportKeepsOtherInspectionThroughSeekEOFAndResize(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		first := m.rows[0]
		inspected := m.rows[1]
		if err := s.store.UpsertEpisodes(s.ctx, first.FeedID, []store.Episode{{GUID: "third", Title: "Next current", AudioURL: "https://example.test/third"}}); err != nil {
			t.Fatal(err)
		}
		all, err := s.store.Episodes(s.ctx, first.FeedID)
		if err != nil {
			t.Fatal(err)
		}
		var next int64
		for _, item := range all {
			if item.GUID == "third" {
				next = item.ID
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}))
		if err := s.playback.Enqueue(s.ctx, next); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		deadline := time.Now().Add(time.Second)
		for !syncPlayer(t, &m).CanSeek {
			if time.Now().After(deadline) {
				t.Fatal("not seekable")
			}
			time.Sleep(time.Millisecond)
		}
		m.Model = m.Model.WithFocus(ui.Episodes).Select(ui.Episodes, fmt.Sprint(inspected.ID))
		libraryKey(&m, 'i')
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		if state := syncPlayer(t, &m); state.Status != playback.Paused || state.Current.ID != first.ID {
			t.Fatal(state)
		}
		if err := s.playback.SeekBy(s.ctx, 15); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		for _, size := range [][2]int{{140, 40}, {70, 20}, {30, 12}, {160, 60}} {
			updateLibrary(&m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			if m.detailTarget == nil || m.detailTarget.ID != inspected.ID || !strings.Contains(m.View().Content, inspected.Title) {
				t.Fatal("resize replaced inspection")
			}
		}
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		p.events <- player.Event{Name: "end-file", Reason: "eof", PlaylistEntryID: &entry}
		deadline = time.Now().Add(time.Second)
		for syncPlayer(t, &m).Current.ID != next {
			if time.Now().After(deadline) {
				t.Fatal("EOF did not advance")
			}
			time.Sleep(time.Millisecond)
		}
		if m.detailTarget == nil || m.detailTarget.ID != inspected.ID {
			t.Fatal("EOF replaced reading target")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		if item, _ := m.Model.Selected(ui.Episodes); item.ID != fmt.Sprint(inspected.ID) || m.Model.Focus() != ui.Episodes {
			t.Fatal("lost return selection", item)
		}
	})
}
