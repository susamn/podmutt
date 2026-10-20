package ui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func TestPlayerIdentityUsesFullChannelAndRemainingWidthWithPadding(t *testing.T) {
	for _, names := range [][2]string{{strings.Repeat("A", 80), strings.Repeat("B", 40)}, {strings.Repeat("界", 60), strings.Repeat("語", 30)}} {
		player := Player{HasCurrent: true, Status: "paused", Title: names[0], Channel: names[1], Position: "0:42", Duration: "48:00"}
		m := New(Data{Player: player})
		line := ansi.Strip(m.renderPlayerTopLine(player, 100))
		parts := strings.Split(strings.TrimSpace(line), " · ")
		if len(parts) != 2 || parts[1] != names[1] || ansi.StringWidth(line) != 100 || !strings.HasPrefix(line, "  ") || !strings.HasSuffix(line, "  ") || !utf8.ValidString(line) {
			t.Fatal("full channel/remaining width/padding", line)
		}
		pane := ansi.Strip(m.playerPane(100, 7))
		rows := strings.Split(pane, "\n")
		if len(rows) != 7 || !strings.Contains(rows[2], " · ") || !strings.Contains(rows[0], "Paused") || !strings.Contains(rows[3], "0:42") || !strings.Contains(rows[4], "[Play] [Stop] [Next]") || strings.Trim(rows[1], "│ ") != "" || strings.Trim(rows[5], "│ ") != "" {
			t.Fatal("extra identity line or lost state/transport", pane)
		}
		if m.data.Player.Title != names[0] || m.data.Player.Channel != names[1] {
			t.Fatal("truncation mutated metadata")
		}
	}
	m := New(Data{})
	for _, width := range []int{0, 4, 28, 68, 78} {
		line := m.renderPlayerTopLine(Player{Title: "Episode\x1b[2J identity", Channel: "Channel\nname"}, width)
		if ansi.StringWidth(line) > width || strings.Contains(line, "\x1b[2J") || strings.Contains(line, "\n") {
			t.Fatal("unsafe/overflowing identity", width, line)
		}
	}
}

func TestEpisodeHeadersAndRowsReserveFullDurationAtEverySupportedWidth(t *testing.T) {
	m := New(Data{})
	item := Item{PlainTitle: strings.Repeat("Long title 界 ", 50), Channel: strings.Repeat("Channel 界 ", 30), Date: "2026-10-20", Duration: "1:23:45", Queued: true}
	for _, width := range []int{30, 45, 55, 56, 63, 69, 70, 71, 76, 89, 100, 180} {
		c := episodeColumns(width)
		header := ansi.Strip(m.renderEpisodesHeader(width))
		row := ansi.Strip(m.renderEpisodeRow(item, width, true))
		if ansi.StringWidth(row) != width-2 || ansi.StringWidth(header) != width-2 || strings.TrimSpace(ansi.Cut(row, c.inner-8, c.inner)) != item.Duration || strings.TrimSpace(ansi.Cut(header, c.inner-8, c.inner)) != "Duration" {
			t.Fatal("duration clipped or header/row disagree", width, header, row)
		}
		if c.channel > 20 {
			t.Fatal("wasted channel gap", width, c)
		}
		if c.date > 0 {
			dateStart := c.gutter + c.title + 2 + c.channel + 2
			if ansi.Cut(row, dateStart, dateStart+10) != item.Date || strings.TrimSpace(ansi.Cut(header, dateStart, dateStart+10)) != "Date" {
				t.Fatal("channel/date columns drift", width, header, row)
			}
		}
	}
	if row := ansi.Strip(m.renderEpisodeRow(Item{Title: "Unknown duration"}, 30, false)); !strings.HasSuffix(row, "--:--") {
		t.Fatal("unknown duration vanished", row)
	}
}

func TestQueueHeadersStayPinnedAndAlignWithRowsAndLargeIndices(t *testing.T) {
	var items []Item
	for i := range 1200 {
		items = append(items, Item{ID: fmt.Sprint(i), PlainTitle: "Queued title", Channel: "Show name", PositionSeconds: 25, DurationSeconds: 100})
	}
	m := resize(New(Data{Queue: items}), 140, 40).WithFocus(Queue)
	for _, width := range []int{30, 35, 49, 56, 100} {
		c := queueColumns(width, m.queueGutter())
		header := ansi.Strip(m.renderQueueHeader(width))
		last := m.Visible(Queue)[1199]
		row := ansi.Strip(m.renderQueueRow(last, width, true))
		if !strings.HasSuffix(header, "Remains") || !strings.HasSuffix(row, "75%") || ansi.StringWidth(row) != width-2 || ansi.StringWidth(header) != width-2 {
			t.Fatal("queue headers/data drift", width, header, row)
		}
		if strings.TrimSpace(ansi.Cut(header, c.gutter, c.gutter+c.title)) != "Title" || !strings.HasPrefix(ansi.Cut(row, c.gutter, c.gutter+c.title), "Queued") {
			t.Fatal("index gutter/header/Title disagree", width, header, row)
		}
		if c.channel > 0 && strings.TrimSpace(ansi.Cut(header, c.gutter+c.title+2, c.gutter+c.title+2+c.channel)) != "Channel" {
			t.Fatal("channel header drift", width, header)
		}
	}
	m = press(m, "G")
	pane := ansi.Strip(m.pane(Queue, 49, 9))
	lines := strings.Split(pane, "\n")
	if !strings.Contains(lines[1], "Title") || !strings.Contains(lines[1], "Channel") || !strings.Contains(lines[1], "Remains") || !strings.Contains(pane, "1200.") {
		t.Fatal("scroll moved header or hid selection", pane)
	}
	m = press(m, "g", "j")
	if selected, _ := m.Selected(Queue); selected.ID != "1" {
		t.Fatal("header became selectable", selected)
	}
}
