package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// EpisodeDetails is one presentation contract for cached and preview episodes.
// Key is stable identity, not display text; transport remains in Player.
type EpisodeDetails struct {
	Key, Title, Show, Date, Duration string
	State, Position, Description     string
	Queued, Inspecting               bool
}

func (m Model) WithEpisodeDetails(card EpisodeDetails) Model {
	if m.data.EpisodeDetails != nil && *m.data.EpisodeDetails == card {
		return m
	}
	if m.data.EpisodeDetails == nil || m.data.EpisodeDetails.Key != card.Key {
		m.detailScroll = 0
	}
	m.data.EpisodeDetails = &card
	m.data.Details = ""
	if m.detailScroll > 0 {
		m.scrollDetails(0)
	}
	return m
}

func wrapDetails(text string, width int) []string {
	return strings.Split(ansi.Wrap(clean(text, true), max(1, width), ""), "\n")
}

// Keep a compact identity header and at least one reading row. Values that do
// not fit the header remain accessible in the scrollable text, even at 30x12.
func (m Model) detailLayout(width, height int) (header, body []string) {
	width = max(1, m.descriptionWidth(width)-2)
	card := m.data.EpisodeDetails
	if card == nil {
		text := m.data.Details
		if text == "" {
			text = "Select an episode for details."
		}
		return nil, wrapDetails(text, width)
	}
	pal := m.Theme()
	var overflow []string
	title := clean(card.Title, false)
	titleRows := wrapDetails(title, width)
	for _, row := range titleRows[:min(2, len(titleRows))] {
		header = append(header, lipgloss.NewStyle().Bold(true).Render(row))
	}
	if len(titleRows) > 2 {
		overflow = append(overflow, "Title: "+title)
	}
	add := func(text string, style lipgloss.Style) {
		text = clean(text, false)
		if text == "" {
			return
		}
		header = append(header, style.Render(ansi.Truncate(text, width, "…")))
		if ansi.StringWidth(text) > width {
			overflow = append(overflow, text)
		}
	}
	add(card.Show, lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.ChannelColor(card.Show)))))
	var metadata []string
	for _, field := range []string{card.Date, card.Duration, card.State} {
		if field != "" {
			metadata = append(metadata, field)
		}
	}
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(string(pal.Muted)))
	add(strings.Join(metadata, " · "), muted)
	var progress []string
	if card.Position != "" {
		progress = append(progress, "Progress "+card.Position)
	}
	if card.Queued {
		progress = append(progress, "In Up next")
	}
	add(strings.Join(progress, " · "), muted)
	limit := max(0, height-4)
	if len(header) > limit {
		for _, row := range header[limit:] {
			overflow = append(overflow, ansi.Strip(row))
		}
		header = header[:limit]
	}
	if len(header) > 0 && len(header) < limit {
		header = append(header, "")
	}
	description := clean(card.Description, true)
	if strings.TrimSpace(description) == "" {
		description = "No description provided."
	}
	if len(overflow) > 0 {
		description = strings.Join(overflow, "\n") + "\n\n" + description
	}
	return header, wrapDetails(description, width)
}

func (m Model) detailRows(width int) []string {
	_, height := m.detailsSize()
	_, body := m.detailLayout(width, height)
	return body
}

func (m Model) detailCapacity(width, height int) int {
	header, _ := m.detailLayout(width, height)
	return max(0, height-3-len(header))
}

func (m *Model) scrollDetails(delta int) {
	width, height := m.detailsSize()
	header, body := m.detailLayout(width, height)
	capacity := max(1, height-3-len(header))
	m.detailScroll = max(0, min(m.detailScroll+delta, len(body)-capacity))
}

func (m Model) renderDetails(width, height int) (rows []string, indicator string) {
	header, body := m.detailLayout(width, height)
	capacity := max(0, height-3-len(header))
	start := min(m.detailScroll, max(0, len(body)-max(1, capacity)))
	end := min(len(body), start+capacity)
	rows = append(header, body[start:end]...)
	if capacity > 0 && len(body) > capacity {
		indicator = fmt.Sprintf("%d/%d", end, len(body))
	}
	return rows, indicator
}
