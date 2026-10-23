package app

import (
	"fmt"
	"strconv"

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
	if episode.HasChapters() {
		marker += "[Ch] "
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
	title := plain
	if episode.HasChapters() {
		title = "[Ch] " + plain
	}
	return ui.Item{
		ID:         fmt.Sprintf("preview:%d", index),
		Title:      title,
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
	remains := ui.FormatRemainingPercent(episode.PositionSeconds, float64(episode.DurationSeconds))
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
type queueConfirmation struct {
	ids       []int64 // D clears only this captured pending queue
	episodeID int64   // d/Delete removes only this captured row
	action    string  // "remove_queue", "clear_queue", "favorite", "unfavorite"
}
type queueEdited struct {
	undo  bool
	err   error
	state *playback.State
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
	if m.Model.Focus() == ui.Favorites {
		item, ok := m.Model.Selected(ui.Favorites)
		if !ok {
			return store.Episode{}, false
		}
		for _, episode := range m.snapshot.Favorites {
			if item.ID == strconv.FormatInt(episode.ID, 10) {
				return episode, true
			}
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
	return work(ctx, "Starting playback…", func(ctx context.Context) tea.Msg { return playEpisode(ctx, engine, id, false) })
}

// Return the actor's authoritative membership with the action, not only on a
// later streamed update. Failed loads can also leave visible current audio.
func playEpisode(ctx context.Context, engine *playback.Engine, id int64, cached bool) playerEdited {
	reply := playerEdited{err: engine.PlayEpisode(ctx, id), cached: cached}
	state, err := engine.Snapshot(ctx)
	if err == nil {
		reply.state = &state
	} else if reply.err == nil {
		reply.err = err
	}
	return reply
}

func (m *libraryShell) openDetails() {
	focus := m.Model.Focus()
	if focus != ui.Episodes && focus != ui.Queue && focus != ui.Favorites {
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
	} else if !batch && (m.Model.Focus() == ui.Episodes || m.Model.Focus() == ui.Queue || m.Model.Focus() == ui.Details || m.Model.Focus() == ui.Favorites) {
		if episode, ok := m.contextEpisode(); ok {
			ids = append(ids, episode.ID)
		}
	}
	if len(ids) == 0 {
		m.notice = "Select episodes to enqueue"
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return work(ctx, "Adding episodes to Up next…", func(ctx context.Context) tea.Msg { return enqueueEpisodes(ctx, engine, ids, false) })
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
	label, state := "Remove episode? y/n", "Pending"
	if selected.Current {
		label, state = "Stop & remove? y/n", string(m.state.Status)
	}
	m.confirming = &queueConfirmation{episodeID: episode.ID, action: "remove_queue"}
	m.Model = m.Model.WithDialog(&ui.Dialog{
		Title:        label,
		Body:         fmt.Sprintf("%s\n%s\n%s · %s/%s\nProgress/history kept.", cleanTitle(episode.Title), cleanShow(episode.FeedTitle), state, clock(episode.PositionSeconds), formatEpisodeDuration(episode.DurationSeconds)),
		TruncateBody: true,
		Hint:         "y remove · n/Esc cancel",
	})
	return nil
}

func (m *libraryShell) openQueueActions() tea.Cmd {
	if m.Model.Focus() != ui.Queue || m.queueEditing {
		return nil
	}
	episode, ok := m.contextEpisode()
	if !ok || episode.ID <= 0 {
		m.notice = "Select an episode in Queue first"
		return nil
	}
	isFav := m.snapshot.FavoriteIDs[episode.ID]
	action := "1. Mark as favorite"
	if isFav {
		action = "1. Already in favorites"
	}
	m.confirming = &queueConfirmation{episodeID: episode.ID, action: "favorite"}
	m.Model = m.Model.WithDialog(&ui.Dialog{
		Title: "Episode Actions",
		Body:  cleanTitle(episode.Title) + "\n" + cleanShow(episode.FeedTitle) + "\n\n" + action,
		Hint:  "Enter apply · Esc cancel",
	})
	return nil
}

func (m *libraryShell) removeFavorite() tea.Cmd {
	if m.Model.Focus() != ui.Favorites || m.queueEditing {
		return nil
	}
	episode, ok := m.contextEpisode()
	if !ok || episode.ID <= 0 {
		return nil
	}
	m.confirming = &queueConfirmation{episodeID: episode.ID, action: "unfavorite"}
	m.Model = m.Model.WithDialog(&ui.Dialog{
		Title:        "Remove from favorites? y/n",
		Body:         cleanTitle(episode.Title) + "\n" + cleanShow(episode.FeedTitle) + "\n\nOnly un-favorites; playback, queue, and history are kept.",
		TruncateBody: true,
		Hint:         "y remove · n/Esc cancel",
	})
	return nil
}

func (m *libraryShell) undoQueueRemoval() tea.Cmd {
	if m.queueEditing {
		m.queueUndoPending = true
		return nil
	}
	m.queueEditing = true
	engine, ctx := m.s.playback, m.s.ctx
	return work(ctx, "Restoring episode to Up next…", func(ctx context.Context) tea.Msg { return queueEdited{undo: true, err: engine.UndoQueueRemoval(ctx)} })
}

func (m *libraryShell) confirmClearQueue() {
	if m.Model.Focus() != ui.Queue || len(m.state.Queue) == 0 || m.queueEditing {
		return
	}
	confirmation := &queueConfirmation{action: "clear_queue"}
	for _, episode := range m.state.Queue {
		confirmation.ids = append(confirmation.ids, episode.ID)
	}
	m.confirming = confirmation
	m.Model = m.Model.WithDialog(&ui.Dialog{Title: "Confirm Up next edit", Body: "Clear pending episodes? Current playback and history are unchanged.", Hint: "y confirm · Esc/n cancel · q quit"})
}
func (m *libraryShell) confirmationKey(key tea.KeyPressMsg) tea.Cmd {
	k := key.String()
	if k == "q" || k == "ctrl+c" {
		return tea.Quit
	}
	confirmation := m.confirming
	if confirmation == nil {
		return nil
	}

	if confirmation.action == "favorite" {
		if k == "esc" || k == "n" {
			m.confirming = nil
			m.Model = m.Model.WithDialog(nil)
			return nil
		}
		if k == "enter" || k == "1" || k == "y" {
			m.confirming = nil
			m.Model = m.Model.WithDialog(nil)
			if m.snapshot.FavoriteIDs[confirmation.episodeID] {
				m.notice = "Already in favorites"
				return nil
			}
			service, ctx, id := m.service, m.s.ctx, confirmation.episodeID
			return work(ctx, "Adding to favorites…", func(ctx context.Context) tea.Msg {
				err := service.AddFavorite(ctx, id)
				return favoriteEdited{added: true, err: err}
			})
		}
		return nil
	}

	if confirmation.action == "unfavorite" {
		if k != "y" && k != "esc" && k != "n" {
			return nil
		}
		m.confirming = nil
		m.Model = m.Model.WithDialog(nil)
		if k != "y" {
			m.notice = "Favorites unchanged"
			return nil
		}
		service, ctx, id := m.service, m.s.ctx, confirmation.episodeID
		return work(ctx, "Removing from favorites…", func(ctx context.Context) tea.Msg {
			err := service.RemoveFavorite(ctx, id)
			return favoriteEdited{added: false, err: err}
		})
	}

	if k != "y" && k != "esc" && k != "n" {
		return nil
	}
	m.confirming = nil
	m.Model = m.Model.WithDialog(nil)
	if k != "y" {
		m.notice = "Up next unchanged"
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	if confirmation.episodeID != 0 {
		m.queueEditing = true
		return work(ctx, "Removing episode from Queue…", func(ctx context.Context) tea.Msg {
			reply := queueEdited{err: engine.RemoveQueueItemUndoable(ctx, confirmation.episodeID)}
			state, err := engine.Snapshot(ctx)
			if err == nil {
				reply.state = &state
			} else if reply.err == nil {
				reply.err = err
			}
			return reply
		})
	}
	return work(ctx, "Clearing Up next…", func(ctx context.Context) tea.Msg { return playerEdited{err: engine.ClearQueue(ctx, confirmation.ids)} })
}
