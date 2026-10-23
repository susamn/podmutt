package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

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
			if c.chapter != 2 || ansi.Cut(header, start, start+2) != "Ch" || ansi.Cut(row, start, start+2) != "📖" || ansi.StringWidth(row) != width-2 {
				t.Fatalf("%v width %d: header=%q row=%q layout=%+v", panel, width, header, row, c)
			}
			item.Chapters = false
			if without := ansi.Strip(m.renderRow(panel, item, width, false)); strings.Contains(without, "📖") || ansi.Cut(without, start, start+2) != "  " {
				t.Fatal("unknown chapters had icon or shifted text", without)
			}
			m.icons = "plain"
			item.Chapters = true
			if plain := ansi.Strip(m.renderRow(panel, item, width, false)); ansi.Cut(plain, start, start+2) != "Ch" {
				t.Fatal("plain fallback missing", plain)
			}
			m.icons = ""
		}
	}
}
