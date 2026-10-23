package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestChapterColumnPersistsWhilePanningTitle(t *testing.T) {
	m := New(Data{})
	item := Item{Title: "Backup title", Chapters: true, Duration: "3:00"}
	for _, panel := range []Panel{Episodes, Queue} {
		m.colOffset[panel] = 2
		var row string
		if panel == Episodes {
			row = ansi.Strip(m.renderEpisodeColumns(item, 55))
		} else {
			row = ansi.Strip(m.renderQueueColumns(item, 55, "45%"))
		}
		if !strings.Contains(row, "ckup title") || !strings.Contains(row, " ✓") {
			t.Fatal("panning moved chapter glyph", panel, row)
		}
		m.colOffset[panel] = 100
		if panel == Episodes {
			row = ansi.Strip(m.renderEpisodeColumns(item, 55))
		} else {
			row = ansi.Strip(m.renderQueueColumns(item, 55, "45%"))
		}
		if strings.Contains(row, "Backup") || !strings.Contains(row, " ✓") {
			t.Fatal("panning hid chapter glyph", panel, row)
		}
	}
}

func TestChapterColumnOnlyShrinksTitleAndAlignsQueueAndEpisodes(t *testing.T) {
	m := New(Data{})
	for _, width := range []int{30, 35, 55, 56, 69, 70, 100, 160} {
		for _, panel := range []Panel{Episodes, Queue} {
			item := Item{PlainTitle: strings.Repeat("Long title ", 12), Channel: "Channel", Date: "2026-03-03", Duration: "1:04:30", DurationSeconds: 3870, PositionSeconds: 100, Chapters: true}
			var c listColumns
			var header, row string
			if panel == Episodes {
				c = episodeColumns(width)
				header, row = ansi.Strip(m.renderEpisodesHeader(width)), ansi.Strip(m.renderEpisodeRow(item, width, false))
			} else {
				c = queueColumns(width, m.queueGutter())
				header, row = ansi.Strip(m.renderQueueHeader(width)), ansi.Strip(m.renderQueueRow(item, width, false))
			}
			start := c.gutter + c.title + 2
			if c.chapter != 2 || ansi.Cut(header, start, start+2) != "Ch" || ansi.Cut(row, start, start+2) != " ✓" || ansi.StringWidth(row) != width-2 {
				t.Fatalf("%v width %d: header=%q row=%q layout=%+v", panel, width, header, row, c)
			}
			raw := m.renderRow(panel, item, width, false)
			if !strings.Contains(raw, m.chapterCell(item)) || ansi.StringWidth(m.chapterCell(item)) != 1 {
				t.Fatal("tick missing accent styling or not a one-cell glyph", raw)
			}
			item.Chapters = false
			if without := ansi.Strip(m.renderRow(panel, item, width, false)); strings.Contains(without, "✓") || ansi.Cut(without, start, start+2) != "  " {
				t.Fatal("unknown chapters had icon or shifted text", without)
			}
			m.icons = "plain"
			item.Chapters = true
			if plain := ansi.Strip(m.renderRow(panel, item, width, false)); ansi.Cut(plain, start, start+2) != " ✓" {
				t.Fatal("plain mode lost the tick", plain)
			}
			m.icons = ""
		}
	}
}
