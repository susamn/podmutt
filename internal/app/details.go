package app

import (
	"fmt"

	"podmutt/internal/library"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

func episodeDetails(episode store.Episode, show, key string, queued, inspecting bool) ui.EpisodeDetails {
	card := ui.EpisodeDetails{
		Key: key, Title: cleanTitle(episode.Title), Show: cleanShow(show),
		Date: formatPublishedDate(episode.PublishedAt), State: "Not started",
		Description: library.PlainText(episode.Description), Queued: queued, Inspecting: inspecting,
	}
	if episode.DurationSeconds > 0 {
		card.Duration = formatEpisodeDuration(episode.DurationSeconds)
	}
	if episode.Completed {
		card.State = "Completed"
	} else if episode.PositionSeconds > 0 {
		card.State, card.Position = "In progress", clock(episode.PositionSeconds)
	}
	return card
}

// Resolve history without replacing inspected RSS metadata or doing database/
// network reads during rendering. Actor state wins over cached history.
func (m libraryShell) detailHistory(episode store.Episode) store.Episode {
	feedID := episode.FeedID
	if episode.ID == 0 && m.browse.preview != nil {
		for _, source := range m.snapshot.Feeds {
			if source.URL == m.browse.preview.URL {
				feedID = source.ID
				break
			}
		}
	}
	var current []store.Episode
	if m.state.Current != nil {
		current = []store.Episode{*m.state.Current}
	}
	for _, group := range [][]store.Episode{current, m.state.Queue, m.snapshot.All, m.snapshot.Continue, m.snapshot.Show} {
		for _, saved := range group {
			matches := episode.ID > 0 && episode.ID == saved.ID
			if episode.ID == 0 && feedID > 0 && feedID == saved.FeedID {
				matches = episode.GUID != "" && episode.GUID == saved.GUID || episode.GUID == "" && episode.AudioURL == saved.AudioURL
			}
			if matches {
				episode.ID, episode.PositionSeconds, episode.Completed = saved.ID, saved.PositionSeconds, saved.Completed
				return episode
			}
		}
	}
	return episode
}

func (m *libraryShell) renderDetails() {
	inspecting := m.Model.Focus() == ui.Details && m.detailTarget != nil
	if !inspecting {
		m.detailTarget = nil
	}
	episode, ok := m.contextEpisode()
	if inspecting {
		episode, ok = *m.detailTarget, true
	}
	if !ok {
		if m.browse.active && m.Model.Focus() != ui.Queue {
			m.renderPreviewDetails()
		} else {
			m.Model = m.Model.WithDetails("")
		}
		return
	}
	show, key := episode.FeedTitle, fmt.Sprintf("episode:%d", episode.ID)
	if episode.ID == 0 && m.browse.preview != nil {
		show = m.browse.preview.Fetched.Document.Title
		key = m.browse.preview.URL + "\x00" + episode.GUID + "\x00" + episode.AudioURL
	} else if show == "" {
		for _, source := range m.snapshot.Feeds {
			if source.ID == episode.FeedID {
				show = source.Title
				break
			}
		}
	}
	episode = m.detailHistory(episode)
	queued := false
	for _, item := range m.state.Queue {
		if episode.ID > 0 && item.ID == episode.ID {
			queued = true
			break
		}
	}
	m.Model = m.Model.WithEpisodeDetails(episodeDetails(episode, show, key, queued, inspecting))
}
