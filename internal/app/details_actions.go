package app

import (
	"fmt"

	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func (m libraryShell) previewActionContext() bool {
	focus := m.Model.Focus()
	return m.browse.active && (focus == ui.Episodes || focus == ui.Details && m.detailTarget != nil && m.detailTarget.ID == 0)
}

func (m libraryShell) previewTargetIndex(episode store.Episode) int {
	if m.browse.preview != nil {
		for i, item := range m.browse.preview.Fetched.Document.Episodes {
			if item.GUID == episode.GUID && item.AudioURL == episode.AudioURL {
				return i
			}
		}
	}
	return -1
}

func (m *libraryShell) leaveDetails() {
	origin, target := m.detailFrom, m.detailTarget
	m.Model = m.Model.WithFocus(origin)
	if target != nil && (origin == ui.Episodes || origin == ui.Queue) {
		id := fmt.Sprint(target.ID)
		if target.ID == 0 {
			id = fmt.Sprintf("preview:%d", m.previewTargetIndex(*target))
		}
		m.Model = m.Model.Select(origin, id)
	}
	m.renderDetails()
}

func (m *libraryShell) renderEpisodeActions() {
	episode, ok := m.contextEpisode()
	if ok && m.previewActionContext() && m.Model.Focus() == ui.Details {
		ok = m.detailTarget != nil && m.previewTargetIndex(*m.detailTarget) >= 0
	}
	actions := ui.EpisodeActions{Available: ok}
	if ok {
		episode = m.detailHistory(episode)
		actions.Resume = !episode.Completed && episode.PositionSeconds > 0
		actions.CanComplete = !m.previewActionContext() && episode.ID > 0
		actions.Completed = episode.Completed
		actions.Current = m.state.Current != nil && m.state.Current.ID == episode.ID && !m.state.Current.Completed
		for _, item := range m.state.Queue {
			if item.ID == episode.ID && episode.ID > 0 {
				actions.Queued = true
				break
			}
		}
	}
	m.Model = m.Model.WithEpisodeActions(actions)
}
