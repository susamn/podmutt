package app

import (
	"fmt"
	"math"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/library"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func queueItem(episode store.Episode) ui.Item {
	show := library.PlainText(episode.FeedTitle)
	if show == "" {
		show = "Unknown show"
	}
	duration := "--:--"
	if episode.DurationSeconds > 0 {
		duration = clock(float64(episode.DurationSeconds))
	}
	plainTitle := library.PlainText(episode.Title)
	if plainTitle == "" {
		plainTitle = "Untitled episode"
	}
	pos := clock(episode.PositionSeconds)
	remains := "100%"
	if episode.DurationSeconds > 0 {
		if episode.PositionSeconds > 0 {
			pct := int(math.Round((1.0 - (episode.PositionSeconds / float64(episode.DurationSeconds))) * 100))
			pct = max(0, min(100, pct))
			remains = fmt.Sprintf("%d%%", pct)
		}
	}
	return ui.Item{
		ID:              strconv.FormatInt(episode.ID, 10),
		Title:           fmt.Sprintf("%s · %s · %s/%s", plainTitle, show, pos, duration),
		PlainTitle:      plainTitle,
		Channel:         show,
		Position:        pos,
		Duration:        duration,
		Remains:         remains,
		PositionSeconds: episode.PositionSeconds,
		DurationSeconds: float64(episode.DurationSeconds),
		Completed:       episode.Completed,
	}
}

type playerEdited struct{ err error }
type queueConfirmation struct{ ids []int64 }
type queueEdited struct {
	undo bool
	err  error
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
	if !selected {
		m.notice = "Select an episode to play"
		return nil
	}
	engine, ctx, id := m.s.playback, m.s.ctx, episode.ID
	return func() tea.Msg { return playerEdited{engine.PlayEpisode(ctx, id)} }
}

func (m *libraryShell) openDetails() {
	focus := m.Model.Focus()
	if focus != ui.Episodes && focus != ui.Queue {
		return
	}
	if episode, ok := m.contextEpisode(); ok {
		m.detailTarget, m.detailFrom = &episode, focus
		m.Model = m.Model.WithFocus(ui.Details)
		m.renderDetails()
	}
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
func (m *libraryShell) removeQueueItem() tea.Cmd {
	if m.Model.Focus() != ui.Queue || m.queueEditing {
		return nil
	}
	episode, ok := m.contextEpisode()
	if !ok {
		return nil
	}
	m.queueEditing = true
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return queueEdited{err: engine.RemoveQueuedUndoable(ctx, episode.ID)} }
}

func (m *libraryShell) undoQueueRemoval() tea.Cmd {
	if m.queueEditing {
		m.queueUndoPending = true
		return nil
	}
	m.queueEditing = true
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return queueEdited{undo: true, err: engine.UndoQueueRemoval(ctx)} }
}

func (m *libraryShell) confirmClearQueue() {
	if m.Model.Focus() != ui.Queue || len(m.state.Queue) == 0 || m.queueEditing {
		return
	}
	confirmation := &queueConfirmation{}
	for _, episode := range m.state.Queue {
		confirmation.ids = append(confirmation.ids, episode.ID)
	}
	m.confirming = confirmation
	m.Model = m.Model.WithDialog(&ui.Dialog{Title: "Confirm Up next edit", Body: "Clear pending episodes? Current playback and history are unchanged.", Hint: "y confirm · Esc/n cancel · q quit"})
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
		m.notice = "Up next unchanged"
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return playerEdited{engine.ClearQueue(ctx, confirmation.ids)} }
}
