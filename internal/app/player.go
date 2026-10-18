package app

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

type playerEdited struct{ err error }
type queueConfirmation struct {
	ids   []int64
	clear bool
}

func clock(seconds float64) string {
	value := int64(max(0, seconds))
	if value >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", value/3600, value/60%60, value%60)
	}
	return fmt.Sprintf("%d:%02d", value/60, value%60)
}
func (m libraryShell) contextEpisode() (store.Episode, bool) {
	if m.Model.Focus() == ui.Queue {
		item, ok := m.Model.Selected(ui.Queue)
		if !ok {
			return store.Episode{}, false
		}
		for _, episode := range m.state.Queue {
			if item.ID == strconv.FormatInt(episode.ID, 10) {
				return episode, true
			}
		}
		return store.Episode{}, false
	}
	return m.selectedEpisode()
}
func (m *libraryShell) playSelected() tea.Cmd {
	if m.browse.active && m.Model.Focus() == ui.Episodes {
		return m.previewPlayback(false, false)
	}
	episode, selected := m.contextEpisode()
	selected = selected && (m.Model.Focus() == ui.Episodes || m.Model.Focus() == ui.Queue)
	engine, ctx := m.s.playback, m.s.ctx
	id := int64(0)
	if selected {
		id = episode.ID
	}
	return func() tea.Msg { return playerEdited{engine.Toggle(ctx, id)} }
}
func (m *libraryShell) enqueue(batch bool) tea.Cmd {
	if m.browse.active && m.Model.Focus() == ui.Episodes {
		return m.previewPlayback(batch, true)
	}
	var ids []int64
	if batch && m.Model.Focus() == ui.Episodes {
		for _, item := range m.Model.Visible(ui.Episodes) {
			id, _ := strconv.ParseInt(item.ID, 10, 64)
			ids = append(ids, id)
		}
	} else if !batch && (m.Model.Focus() == ui.Episodes || m.Model.Focus() == ui.Queue) {
		if episode, ok := m.contextEpisode(); ok {
			ids = append(ids, episode.ID)
		}
	}
	if len(ids) == 0 {
		m.notice = "Select episodes to enqueue"
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return playerEdited{engine.Enqueue(ctx, ids...)} }
}
func (m *libraryShell) moveQueue(step int) tea.Cmd {
	if m.Model.Focus() != ui.Queue {
		return nil
	}
	episode, ok := m.contextEpisode()
	if !ok {
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return playerEdited{engine.MoveQueued(ctx, episode.ID, step)} }
}
func (m *libraryShell) confirmQueue(clear bool) {
	if m.Model.Focus() != ui.Queue || len(m.state.Queue) == 0 {
		return
	}
	confirmation := &queueConfirmation{clear: clear}
	body := "Clear all queued episodes? Playback and history are unchanged."
	if clear {
		for _, episode := range m.state.Queue {
			confirmation.ids = append(confirmation.ids, episode.ID)
		}
	} else {
		episode, ok := m.contextEpisode()
		if !ok {
			return
		}
		confirmation.ids = []int64{episode.ID}
		body = "Remove from queue: " + episode.Title + "?"
	}
	m.confirming = confirmation
	m.Model = m.Model.WithDialog(&ui.Dialog{Title: "Confirm queue edit", Body: body, Hint: "y confirm · Esc/n cancel · q quit"})
}
func (m *libraryShell) confirmationKey(key tea.KeyPressMsg) tea.Cmd {
	if key.String() == "q" || key.String() == "ctrl+c" {
		return tea.Quit
	}
	if key.String() != "y" && key.String() != "esc" && key.String() != "n" {
		return nil
	}
	confirmation := m.confirming
	m.confirming = nil
	m.Model = m.Model.WithDialog(nil)
	if key.String() != "y" {
		m.notice = "Queue unchanged"
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg {
		if confirmation.clear {
			return playerEdited{engine.ClearQueue(ctx, confirmation.ids)}
		}
		return playerEdited{engine.RemoveQueued(ctx, confirmation.ids...)}
	}
}
