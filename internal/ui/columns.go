package ui

import (
	"fmt"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Headers and data share exact cell budgets, including every separator. Reserve
// the right-hand value first; surplus space belongs to Title, not channel padding.
type listColumns struct{ inner, gutter, title, chapter, channel, date, tail int }

func episodeColumns(width int) listColumns {
	c := listColumns{inner: max(0, width-2), gutter: 4, tail: 8, chapter: 2}
	body := c.inner - c.gutter - c.tail - 1
	if width >= 70 {
		c.date = 11
		body -= 13
	}
	if width >= 56 {
		c.channel = min(20, max(7, (body-2)/3))
		body -= c.channel + 2
	}
	c.title = max(0, body-c.chapter-2)
	return c
}

func queueColumns(width, gutter int) listColumns {
	c := listColumns{inner: max(0, width-2), gutter: gutter, tail: 7, chapter: 2}
	body := c.inner - c.gutter - c.tail - 1
	if body >= 14 {
		c.channel = min(20, max(7, (body-2)/3))
		body -= c.channel + 2
	}
	c.title = max(0, body-c.chapter-2)
	return c
}

func (c listColumns) row(prefix, title, chapter, channel, date, tail string) string {
	row := padRight(prefix, c.gutter) + padRight(title, c.title)
	if c.chapter > 0 {
		row += "  " + padRight(chapter, c.chapter)
	}
	if c.channel > 0 {
		row += "  " + padRight(channel, c.channel)
	}
	if c.date > 0 {
		row += "  " + padRight(date, c.date)
	}
	return ansi.Truncate(row+" "+padLeft(tail, c.tail), c.inner, "…")
}

func (m Model) channelCell(channel string) string {
	channel = clean(channel, false)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(m.Theme().ChannelColor(channel)))).Render(channel)
}

// The fixed two-cell Ch column is present even when an episode is not known
// to have chapters. Never borrow width from Channel, Date, or the right tail.
func (m Model) chapterCell(item Item) string {
	if !item.Chapters {
		return ""
	}
	if m.icons == "plain" {
		return "Ch"
	}
	return "§"
}

func (m Model) queueGutter() int { return max(5, 4+len(strconv.Itoa(max(1, len(m.data.Queue))))) }

func (m Model) renderQueueHeader(width int) string {
	c := queueColumns(width, m.queueGutter())
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(m.Theme().LightForeground))).Render(c.row("", "Title", "Ch", "Channel", "", "Remains"))
}

func (m Model) renderEpisodeColumns(item Item, width int) string {
	gutter := "◉ "
	if item.Queued {
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(m.Theme().Accent))).Render("+ ")
	} else if item.Completed {
		gutter = lipgloss.NewStyle().Foreground(lipgloss.Color(string(m.Theme().Green))).Render("◉ ")
	}
	title := clean(item.PlainTitle, false)
	if title == "" {
		title = clean(item.Title, false)
	}
	if offset := m.colOffset[Episodes]; offset > 0 {
		runes := []rune(title)
		if offset < len(runes) {
			title = string(runes[offset:])
		} else {
			title = ""
		}
	}
	duration := clean(item.Duration, false)
	if duration == "" {
		duration = "--:--"
	}
	return episodeColumns(width).row("  "+gutter, title, m.chapterCell(item), m.channelCell(item.Channel), clean(item.Date, false), duration)
}

func (m Model) renderQueueColumns(item Item, width int, remaining string) string {
	prefix := "  "
	if item.Index > 0 {
		prefix += fmt.Sprintf("%d. ", item.Index)
	}
	title := clean(item.PlainTitle, false)
	if title == "" {
		title = clean(item.Title, false)
	}
	if offset := m.colOffset[Queue]; offset > 0 {
		runes := []rune(title)
		if offset < len(runes) {
			title = string(runes[offset:])
		} else {
			title = ""
		}
	}
	c := queueColumns(width, max(m.queueGutter(), ansi.StringWidth(prefix)))
	if c.inner < c.gutter+c.tail+1 {
		return padLeft(remaining, c.inner)
	}
	return c.row(prefix, title, m.chapterCell(item), m.channelCell(item.Channel), "", remaining)
}

func favoritesColumns(width int) listColumns {
	c := listColumns{inner: max(0, width-2), gutter: 4}
	body := c.inner - c.gutter - 1
	if width >= 50 {
		c.date = 11
		body -= 13
	}
	if width >= 40 {
		c.channel = min(20, max(7, (body-2)/3))
		body -= c.channel + 2
	}
	c.title = max(0, body)
	return c
}

func (m Model) renderFavoritesHeader(width int) string {
	c := favoritesColumns(width)
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(string(m.Theme().LightForeground))).Render(c.row("", "Title", "", "Channel", "Date", ""))
}

func (m Model) renderFavoritesRow(item Item, width int, selected bool) string {
	c := favoritesColumns(width)
	gutter := "♥ "
	if m.icons == "plain" {
		gutter = "* "
	}
	title := clean(item.PlainTitle, false)
	if title == "" {
		title = clean(item.Title, false)
	}
	if offset := m.colOffset[Favorites]; offset > 0 {
		runes := []rune(title)
		if offset < len(runes) {
			title = string(runes[offset:])
		} else {
			title = ""
		}
	}
	gutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(m.Theme().Red)))
	return c.row("  "+gutterStyle.Render(gutter), title, "", m.channelCell(item.Channel), clean(item.Date, false), "")
}
