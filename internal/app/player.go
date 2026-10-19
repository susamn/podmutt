package app

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"context"
	"podmutt/internal/library"
	"podmutt/internal/playback"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func cleanTitle(title string) string {
	plain := library.PlainText(title)
	if plain == "" {
		return "Untitled episode"
	}
	return plain
}

func cleanShow(show string) string {
	plain := library.PlainText(show)
	if plain == "" {
		return "Unknown show"
	}
	return plain
}

func formatPublishedDate(t time.Time) string {
	if t.Unix() > 0 {
		return t.Format("2006-01-02")
	}
	return ""
}

func formatEpisodeDuration(seconds int64) string {
	if seconds > 0 {
		return clock(float64(seconds))
	}
	return "--:--"
}

func episodeItem(episode store.Episode, queued bool) ui.Item {
	plain := cleanTitle(episode.Title)
	marker := "[ ] "
	if episode.Completed {
		marker = "[x] "
	}
	return ui.Item{
		ID:          strconv.FormatInt(episode.ID, 10),
		Title:       marker + plain,
		PlainTitle:  plain,
		Channel:     library.PlainText(episode.FeedTitle),
		Date:        formatPublishedDate(episode.PublishedAt),
		Duration:    formatEpisodeDuration(episode.DurationSeconds),
		Queued:      queued,
		Completed:   episode.Completed,
		PublishedAt: episode.PublishedAt,
	}
}

func previewEpisodeItem(index int, episode store.Episode, showTitle string) ui.Item {
	plain := cleanTitle(episode.Title)
	return ui.Item{
		ID:         fmt.Sprintf("preview:%d", index),
		Title:      plain,
		PlainTitle: plain,
		Channel:    showTitle,
		Date:       formatPublishedDate(episode.PublishedAt),
		Duration:   formatEpisodeDuration(episode.DurationSeconds),
	}
}

func queueItem(episode store.Episode) ui.Item {
	show := cleanShow(episode.FeedTitle)
	duration := formatEpisodeDuration(episode.DurationSeconds)
	plainTitle := cleanTitle(episode.Title)
	pos := clock(episode.PositionSeconds)
	remains := "100%"
	if episode.DurationSeconds > 0 && episode.PositionSeconds > 0 {
		pct := int(math.Round((1.0 - (episode.PositionSeconds / float64(episode.DurationSeconds))) * 100))
		pct = max(0, min(100, pct))
		remains = fmt.Sprintf("%d%%", pct)
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
		PublishedAt:     episode.PublishedAt,
	}
}

type playerEdited struct {
	err      error
	cached   bool
	state    *playback.State
	enqueued []int64
}
type queueConfirmation struct{ ids []int64 }
type queueEdited struct {
	undo bool
	err  error
}

func clock(seconds float64) string {
	return ui.FormatDuration(seconds)
}
func (m libraryShell) contextEpisode() (store.Episode, bool) {
	if m.Model.Focus() == ui.Details {
		if m.detailTarget == nil {
			return store.Episode{}, false
		}
		return m.detailHistory(*m.detailTarget), true
	}
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
		if current := m.state.Current; current != nil && item.ID == strconv.FormatInt(current.ID, 10) {
			return *current, true
		}
		return store.Episode{}, false
	}
	return m.selectedEpisode()
}
func (m *libraryShell) playSelected() tea.Cmd {
	if m.previewActionContext() {
		return m.previewPlayback(false, false)
	}
	episode, selected := m.contextEpisode()
	if !selected {
		m.notice = "Select an episode to play"
		return nil
	}
	engine, ctx, id := m.s.playback, m.s.ctx, episode.ID
	return func() tea.Msg { return playerEdited{err: engine.PlayEpisode(ctx, id)} }
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
	if !batch && m.Model.Focus() == ui.Details {
		if episode, ok := m.contextEpisode(); ok && episode.ID > 0 {
			if m.state.Current != nil && m.state.Current.ID == episode.ID && !m.state.Current.Completed {
				m.notice = "Already now playing"
				return nil
			}
			for _, item := range m.state.Queue {
				if item.ID == episode.ID {
					m.notice = "Already in Up next"
					return nil
				}
			}
		}
	}
	if m.previewActionContext() {
		return m.previewPlayback(batch, true)
	}
	var ids []int64
	if batch && m.Model.Focus() == ui.Episodes {
		for _, item := range m.Model.Visible(ui.Episodes) {
			id, _ := strconv.ParseInt(item.ID, 10, 64)
			ids = append(ids, id)
		}
	} else if !batch && (m.Model.Focus() == ui.Episodes || m.Model.Focus() == ui.Queue || m.Model.Focus() == ui.Details) {
		if episode, ok := m.contextEpisode(); ok {
			ids = append(ids, episode.ID)
		}
	}
	if len(ids) == 0 {
		m.notice = "Select episodes to enqueue"
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return func() tea.Msg { return enqueueEpisodes(ctx, engine, ids, false) }
}
func enqueueEpisodes(ctx context.Context, engine *playback.Engine, ids []int64, cached bool) playerEdited {
	err := engine.Enqueue(ctx, ids...)
	if err != nil {
		return playerEdited{err: err, cached: cached}
	}
	state, err := engine.Snapshot(ctx)
	return playerEdited{err: err, cached: cached, state: &state, enqueued: ids}
}

func (m *libraryShell) removeQueueItem() tea.Cmd {
	if m.Model.Focus() != ui.Queue || m.queueEditing {
		return nil
	}
	episode, ok := m.contextEpisode()
	if !ok {
		return nil
	}
	selected, _ := m.Model.Selected(ui.Queue)
	if selected.Current || !slices.ContainsFunc(m.state.Queue, func(item store.Episode) bool { return item.ID == episode.ID }) {
		m.notice = "Current audio is not a pending queue item"
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
	return func() tea.Msg { return playerEdited{err: engine.ClearQueue(ctx, confirmation.ids)} }
}
