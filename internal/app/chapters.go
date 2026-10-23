package app

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/chapters"
	"podmutt/internal/playback"
	"podmutt/internal/store"
	"podmutt/internal/ui"
)

type chapterRequest struct {
	episodeID  int64
	playbackID uint64
}

func (m *libraryShell) closeChapters() {
	m.chapterRequest = chapterRequest{}
	m.chapterList = nil
	m.Model = m.Model.WithChapters(nil)
}

func (m *libraryShell) showChapters(episode store.Episode, list []chapters.Chapter) {
	rows := make([]ui.ChapterRow, len(list))
	for i, ch := range list {
		rows[i] = ui.ChapterRow{Time: clock(ch.Start), Title: ch.Title}
	}
	_, selected, _ := chapters.Current(list, episode.PositionSeconds)
	m.chapterList = list
	m.Model = m.Model.WithChapters(&ui.ChapterPicker{
		Episode: cleanTitle(episode.Title), Show: cleanShow(episode.FeedTitle),
		Rows: rows, Selected: selected,
	})
}

func (m *libraryShell) openChapters() tea.Cmd {
	current := m.state.Current
	if current == nil {
		m.notice = "No current episode. Play one to browse chapters."
		return nil
	}
	if !m.state.CanSeek || m.state.PlaybackID == 0 {
		m.notice = "Seeking unavailable. Wait for the current episode to load."
		return nil
	}
	m.closeChapters()
	// Chapter files are prefetched during feed refresh. The actor probes embedded
	// metadata on file-loaded. C only reads the cache; never fetches chapters.
	episode := *current
	if saved, err := m.s.store.Episode(m.s.ctx, current.ID); err == nil && saved.AudioURL == current.AudioURL {
		episode.ChaptersURL, episode.ChaptersJSON, episode.ChaptersState = saved.ChaptersURL, saved.ChaptersJSON, saved.ChaptersState
	}
	m.chapterRequest = chapterRequest{episodeID: current.ID, playbackID: m.state.PlaybackID}
	if list := episode.ParsedChapters(); len(list) > 0 {
		m.showChapters(episode, list)
		return nil
	}
	if episode.ChaptersURL != "" {
		switch episode.ChaptersState {
		case "error":
			m.notice = "Published chapter file is unavailable. Its URL must change before it is checked again."
		case "empty":
			m.notice = "No chapters found in the published chapter file."
		default:
			m.notice = "Chapter file is being checked during feed refresh. Try C again shortly."
		}
		return nil
	}
	switch episode.ChaptersState {
	case "embedded-empty":
		m.notice = "No chapters found in the current audio."
	case "embedded-error":
		m.notice = "Embedded chapter data is unavailable for this audio."
	default:
		m.notice = "Checking embedded chapters in current audio. Try C again shortly."
	}
	return nil
}

func (m *libraryShell) chapterKey(key string) tea.Cmd {
	switch key {
	case "esc":
		m.closeChapters()
	case "j", "down":
		m.Model = m.Model.MoveChapter(1)
	case "k", "up":
		m.Model = m.Model.MoveChapter(-1)
	case "pgdown", "ctrl+d":
		m.Model = m.Model.MoveChapter(m.Model.ChapterPageSize())
	case "pgup", "ctrl+u":
		m.Model = m.Model.MoveChapter(-m.Model.ChapterPageSize())
	case "g", "home":
		m.Model = m.Model.MoveChapter(-len(m.chapterList))
	case "G", "end":
		m.Model = m.Model.MoveChapter(len(m.chapterList))
	case "enter":
		selected := m.Model.ChapterSelection()
		if selected < 0 || selected >= len(m.chapterList) {
			return nil
		}
		request, start := m.chapterRequest, m.chapterList[selected].Start
		m.closeChapters()
		engine, ctx := m.s.playback, m.s.ctx
		return work(ctx, "Jumping to chapter…", func(ctx context.Context) tea.Msg {
			return playerEdited{err: engine.SeekChapter(ctx, request.episodeID, request.playbackID, start)}
		})
	}
	return nil
}

// Explicit actor snapshots, not the selected library row, define chapter identity.
func chapterPlaybackChanged(old chapterRequest, state playback.State) bool {
	return old.episodeID != 0 && (state.Current == nil || state.Current.ID != old.episodeID || state.PlaybackID != old.playbackID || !state.CanSeek)
}
