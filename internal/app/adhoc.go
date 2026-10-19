package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/ui"
)

func (m *libraryShell) previewPlayback(batch, enqueue bool) tea.Cmd {
	if m.browse.preview == nil {
		m.notice = "Select an episode from a loaded preview"
		return nil
	}
	items := m.Model.Visible(ui.Episodes)
	if !batch {
		selected, ok := m.Model.Selected(ui.Episodes)
		if !ok {
			m.notice = "Select a preview episode"
			return nil
		}
		items = []ui.Item{selected}
	}
	indices := make([]int, 0, len(items))
	for _, item := range items {
		prefix, ok := strings.CutPrefix(item.ID, "preview:")
		if !ok {
			continue
		}
		index, err := strconv.Atoi(prefix)
		if err == nil {
			indices = append(indices, index)
		}
	}
	if len(indices) == 0 {
		m.notice = "Select a preview episode"
		return nil
	}
	preview, service, engine, ctx := *m.browse.preview, m.service, m.s.playback, m.s.ctx
	return func() tea.Msg {
		ids, err := service.CachePreviewEpisodes(ctx, preview, indices)
		if err != nil {
			return playerEdited{err}
		}
		if enqueue {
			return playerEdited{engine.Enqueue(ctx, ids...)}
		}
		return playerEdited{engine.PlayEpisode(ctx, ids[0])}
	}
}
