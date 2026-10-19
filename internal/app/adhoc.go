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
	var indices []int
	if m.Model.Focus() == ui.Details {
		if batch || m.detailTarget == nil {
			m.notice = "Inspect a preview episode first"
			return nil
		}
		index := m.previewTargetIndex(*m.detailTarget)
		if index < 0 {
			m.notice = "Preview changed · inspect the episode again"
			return nil
		}
		indices = append(indices, index)
		items = nil
	} else if !batch {
		selected, ok := m.Model.Selected(ui.Episodes)
		if !ok {
			m.notice = "Select a preview episode"
			return nil
		}
		items = []ui.Item{selected}
	}
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
			return playerEdited{err: err}
		}
		if enqueue {
			return enqueueEpisodes(ctx, engine, ids, true)
		}
		return playerEdited{err: engine.PlayEpisode(ctx, ids[0]), cached: true}
	}
}
