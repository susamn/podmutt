package ui

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"
)

// ChapterRow is a display-only chapter; playback identities remain in app.
type ChapterRow struct {
	Time, Title string
}

type ChapterPicker struct {
	Episode, Show string
	Rows          []ChapterRow
	Selected      int
}

func (m Model) WithChapters(picker *ChapterPicker) Model {
	m.chapters = picker
	return m
}

func (m Model) ChapterSelection() int {
	if m.chapters == nil {
		return -1
	}
	return m.chapters.Selected
}

func (m Model) MoveChapter(delta int) Model {
	if m.chapters != nil && len(m.chapters.Rows) > 0 {
		copy := *m.chapters
		copy.Selected = max(0, min(len(copy.Rows)-1, copy.Selected+delta))
		m.chapters = &copy
	}
	return m
}

func (m Model) ChapterPageSize() int {
	return max(1, min(22, m.height-statusRows-2)-5)
}

func (m Model) renderChapters(width, height int) string {
	p := m.chapters
	if p == nil {
		return ""
	}
	capacity := max(1, height-5)
	start := max(0, p.Selected-capacity+1)
	if start > 0 && start+capacity > len(p.Rows) {
		start = max(0, len(p.Rows)-capacity)
	}
	rows := []string{ansi.Truncate(clean(p.Episode, false), width-4, "…"), ansi.Truncate(clean(p.Show, false), width-4, "…")}
	for i := start; i < min(len(p.Rows), start+capacity); i++ {
		row := p.Rows[i]
		prefix := "  "
		if i == p.Selected {
			prefix = "▸ "
		}
		line := fmt.Sprintf("%s%s  %s", prefix, row.Time, row.Title)
		rows = append(rows, ansi.Truncate(clean(line, false), width-4, "…"))
	}
	rows = append(rows, ansi.Truncate(fmt.Sprintf("%d/%d · j/k arrows · PgUp/PgDn · Enter jump · Esc cancel", p.Selected+1, len(p.Rows)), width-4, "…"))
	return m.box("Chapters · Current playback", rows, width, height, true, "", "")
}
