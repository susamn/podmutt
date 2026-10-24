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

func (m Model) chapterModalHeight() int {
	return min(22, m.height-statusRows-2)
}

func chapterPageSize(height int) int {
	if height < 12 {
		return max(1, height-5) // episode + show, list, navigation
	}
	return max(1, height-8) // label + episode + show + gap, list, gap + navigation
}

func (m Model) ChapterPageSize() int {
	return chapterPageSize(m.chapterModalHeight())
}

func (m Model) renderChapters(width, height int) string {
	p := m.chapters
	if p == nil {
		return ""
	}
	inner := max(1, width-2)
	textWidth := max(1, inner-3) // two left cells and one right cell
	row := func(text string) string {
		return "  " + ansi.Truncate(text, textWidth, "…")
	}
	line := func(text string) string { return row(clean(text, false)) }
	pal := m.Theme()
	compact := height < 12
	var rows []string
	if !compact {
		rows = append(rows, row(popupSection("NOW PLAYING", pal)))
	}
	rows = append(rows,
		row(popupLabel("Episode:", pal)+" "+clean(p.Episode, false)),
		row(popupLabel("Show:", pal)+" "+clean(p.Show, false)))
	if !compact {
		rows = append(rows, row(popupSection("Chapters", pal)))
	}

	capacity := chapterPageSize(height)
	start := (p.Selected / capacity) * capacity
	for i := start; i < min(len(p.Rows), start+capacity); i++ {
		ch := p.Rows[i]
		mark := "  "
		if i == p.Selected {
			mark = "▸ "
		}
		text := ansi.Truncate(clean(fmt.Sprintf("%s%8s  %s", mark, ch.Time, ch.Title), false), textWidth, "…")
		if i == p.Selected {
			rows = append(rows, "  "+m.highlightRow(text, textWidth))
		} else {
			rows = append(rows, line(fmt.Sprintf("%s%8s  %s", mark, ch.Time, ch.Title)))
		}
	}
	if !compact {
		rows = append(rows, "")
	}
	hint := "j/k · Enter jump · Esc"
	if !compact {
		hint = "j/k or arrows · PgUp/PgDn · Enter jump · Esc back"
	}
	rows = append(rows, row(popupHint(hint, pal)))
	tag := ""
	if width >= 30 {
		tag = fmt.Sprintf("%d/%d", p.Selected+1, len(p.Rows))
	}
	return m.box("Chapters · Current playback", rows, width, height, true, "", tag)
}
