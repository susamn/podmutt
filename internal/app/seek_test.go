package app

import (
	"encoding/json"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/player"
	"podmutt/internal/ui"
)

func TestPlayerSeekUsesActorAndPreservesPause(t *testing.T) {
	librarySession(t, func(s *session) {
		m, p := playerLibrary(t, s)
		finishLibrary(&m, updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeySpace}))
		entry, _ := p.EntryID(s.ctx)
		p.events <- player.Event{Name: "start-file", PlaylistEntryID: &entry}
		p.events <- player.Event{Name: "file-loaded"}
		p.events <- player.Event{Name: "property-change", Property: "time-pos", Data: json.RawMessage(`42`)}
		until := time.Now().Add(time.Second)
		for !syncPlayer(t, &m).CanSeek && time.Now().Before(until) {
			time.Sleep(time.Millisecond)
		}
		if !m.state.CanSeek {
			t.Fatal("loaded player cannot seek")
		}
		if err := s.playback.Pause(s.ctx); err != nil {
			t.Fatal(err)
		}
		syncPlayer(t, &m)
		m.Model = m.Model.WithFocus(ui.PlayerPanel)
		finishLibrary(&m, libraryKey(&m, 'h'))
		if state := syncPlayer(t, &m); state.Current.PositionSeconds != 27 || state.Status != "paused" {
			t.Fatal("backward seek", state)
		}
		finishLibrary(&m, libraryKey(&m, 'l'))
		if state := syncPlayer(t, &m); state.Current.PositionSeconds != 42 || state.Status != "paused" {
			t.Fatal("forward seek", state)
		}
		if m.Model.Focus() != ui.PlayerPanel {
			t.Fatal("seek switched panels")
		}
	})
}
