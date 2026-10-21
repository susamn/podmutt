package app

import (
	"context"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

type libraryStartup struct{}

// Saved work remains accessible without a separate Continue destination or
// implicitly subscribing a preview. Merely fetched/unplayed previews stay out.
func (m libraryShell) allEpisodes() []store.Episode {
	result := append([]store.Episode(nil), m.snapshot.All...)
	seen := make(map[int64]bool, len(result))
	for _, episode := range result {
		seen[episode.ID] = true
	}
	add := func(episode store.Episode) {
		if episode.ID > 0 && !seen[episode.ID] {
			result = append(result, episode)
			seen[episode.ID] = true
		}
	}
	for _, episode := range m.snapshot.Continue {
		add(episode)
	}
	for _, episode := range m.state.Queue {
		add(episode)
	}
	if m.state.Current != nil {
		add(*m.state.Current)
	}
	return result
}

func (m *libraryShell) alignStartup() {
	current := m.state.Current
	if current == nil || current.Completed {
		return
	}
	m.browse.active = false
	m.scopeCurrent()
	m.renderLibrary()
	m.selectCurrent()
}

func (m *libraryShell) scopeCurrent() {
	current := m.state.Current
	m.libraryID = allDestination
	for _, source := range m.snapshot.Feeds {
		if source.ID == current.FeedID && source.Subscribed {
			m.libraryID = strconv.FormatInt(source.ID, 10)
			m.snapshot.Show = nil
			for _, episode := range m.allEpisodes() {
				if episode.FeedID == source.ID {
					m.snapshot.Show = append(m.snapshot.Show, episode)
				}
			}
			break
		}
	}
	m.wantLibraryID = m.libraryID
}

func (m *libraryShell) selectCurrent() {
	if m.state.Current == nil || m.Model.Busy() {
		return
	}
	id := strconv.FormatInt(m.state.Current.ID, 10)
	m.Model = m.Model.Select(ui.Queue, id)
	if !m.browse.active {
		found := false
		for _, episode := range m.rows {
			if episode.ID == m.state.Current.ID {
				found = true
				break
			}
		}
		if !found && !m.Model.HasFilter(ui.Podcasts) && !m.Model.HasFilter(ui.Episodes) {
			m.scopeCurrent()
			m.renderLibrary()
		}
		m.Model = m.Model.Select(ui.Episodes, id)
	}
}

func (m *libraryShell) resumeStartup() tea.Cmd {
	if m.startupDone {
		return nil
	}
	m.startupDone = true
	if m.state.Current == nil || m.state.Current.Completed {
		return nil
	}
	engine, ctx := m.s.playback, m.s.ctx
	return work(ctx, "Resuming playback…", func(ctx context.Context) tea.Msg { return playerEdited{err: engine.Play(ctx)} })
}

func (m *libraryShell) applyQueueFocus() {
	if m.queueSelectID == "" || m.Model.Busy() {
		return
	}
	id := m.queueSelectID
	m.queueSelectID = ""
	visible := m.Model.ClearFilter(ui.Queue)
	for _, item := range visible.Visible(ui.Queue) {
		if item.ID == id {
			m.Model = visible.Select(ui.Queue, id).WithFocus(ui.Queue)
			m.renderDetails()
			return
		}
	}
}
