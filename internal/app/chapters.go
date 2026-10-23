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
	url        string
}
type chapterLoaded struct {
	generation uint64
	request    chapterRequest
	list       []chapters.Chapter
	err        error
}

func (m *libraryShell) closeChapters() {
	if m.chapterCancel != nil {
		m.chapterCancel()
		m.chapterCancel = nil
	}
	m.chapterGeneration++
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
	m.chapterRequest = chapterRequest{episodeID: current.ID, playbackID: m.state.PlaybackID, url: current.ChaptersURL}
	list := current.ParsedChapters()
	if len(list) > 0 {
		m.showChapters(*current, list)
		return nil
	}
	ctx, cancel := context.WithCancel(m.s.ctx)
	m.chapterCancel = cancel
	generation, request, refresher, engine := m.chapterGeneration, m.chapterRequest, m.refresher, m.s.playback
	return work(ctx, "Loading chapters…", func(ctx context.Context) tea.Msg {
		var list []chapters.Chapter
		var err error
		if request.url == "" {
			list, err = engine.EmbeddedChapters(ctx, request.episodeID, request.playbackID)
		} else {
			list, err = refresher.FetchChapters(ctx, request.url)
			if err == nil && len(list) > 0 {
				err = engine.CacheChapters(ctx, request.episodeID, request.playbackID, request.url, list)
			}
		}
		return chapterLoaded{generation: generation, request: request, list: list, err: err}
	})
}

func (m *libraryShell) applyChapters(msg chapterLoaded) {
	if msg.generation != m.chapterGeneration || msg.request != m.chapterRequest {
		return
	}
	m.chapterCancel = nil
	current := m.state.Current
	if current == nil || current.ID != msg.request.episodeID || m.state.PlaybackID != msg.request.playbackID || !m.state.CanSeek {
		m.closeChapters()
		return
	}
	if msg.err != nil {
		if msg.err == playback.ErrSeekUnavailable {
			m.closeChapters()
			return
		}
		m.notice = "Could not load chapters. Check your connection or retry with C."
		return
	}
	if len(msg.list) == 0 {
		m.notice = "No chapters found for the current episode."
		if msg.request.url != "" {
			m.notice = "No chapters found in the published chapter file."
		}
		return
	}
	current.ChaptersJSON, _ = chapters.Serialize(msg.list)
	m.showChapters(*current, msg.list)
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
