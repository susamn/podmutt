package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEpisodeCardPinsIdentityAndPreservesReadingAcrossUpdates(t *testing.T) {
	for _, size := range [][2]int{{140, 56}, {100, 24}, {70, 20}, {30, 12}} {
		for _, icons := range []string{"plain", "unicode", "nerd"} {
			card := EpisodeDetails{Key: "episode:1", Title: "Inspected episode", Show: "RSS show", Date: "2026-04-12", Duration: "48:00", State: "In progress", Position: "12:34", Queued: true, Inspecting: true, Description: strings.Repeat("Readable paragraph 界.\n\n", 100)}
			m := resize(New(Data{}), size[0], size[1]).WithIcons(icons).WithFocus(Details).WithEpisodeDetails(card)
			m = press(m, "ctrl+d")
			if m.detailScroll == 0 {
				t.Fatal("description cannot scroll", size)
			}
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "Inspected episode") || !strings.Contains(view, "RSS show") {
				t.Fatal("identity scrolled away", size, view)
			}
			for _, row := range strings.Split(view, "\n") {
				if ansi.StringWidth(row) > size[0] {
					t.Fatal("card overflow", size)
				}
			}
			offset := m.detailScroll
			card.Position, card.Queued = "12:35", false
			m = m.WithPlayback(nil, Player{Title: "Unrelated current audio"}).WithLibrary(Data{}).WithEpisodeDetails(card)
			if m.detailScroll != offset {
				t.Fatal("progress/library update reset reading", size, offset, m.detailScroll)
			}
			card.Key = "episode:2"
			m = m.WithEpisodeDetails(card)
			if m.detailScroll != 0 {
				t.Fatal("new identity inherited scroll")
			}
			m = m.WithDetails("Discovery guidance")
			if m.data.EpisodeDetails != nil {
				t.Fatal("guidance retained old card")
			}
		}
	}
}

func TestEpisodeCardFullMetadataAccessibleAndUnknownsQuiet(t *testing.T) {
	card := EpisodeDetails{Key: "long", Title: strings.Repeat("Long title 界 ", 30), Show: strings.Repeat("Long show ", 20), Date: "2026-04-12", Duration: "1:48:00", State: "In progress", Position: "12:34", Queued: true, Description: "Last description line\x1b[2J"}
	m := resize(New(Data{}), 30, 12).WithEpisodeDetails(card)
	width, height := m.detailsSize()
	header, body := m.detailLayout(width, height)
	all := ansi.Strip(strings.Join(append(header, body...), "\n"))
	for _, field := range []string{"Title:", "Long show", "2026-04-12", "1:48:00", "Progress 12:34", "In Up next", "Last description"} {
		if !strings.Contains(all, field) {
			t.Fatal("metadata inaccessible", field, all)
		}
	}
	if strings.Contains(all, "\x1b") {
		t.Fatal("untrusted terminal escape")
	}
	m = m.WithEpisodeDetails(EpisodeDetails{Key: "empty", Title: "Untitled episode", Show: "Unknown show", State: "Not started"})
	_, body = m.detailLayout(84, 14)
	all = strings.Join(body, "\n")
	if !strings.Contains(all, "No description provided.") || strings.Contains(all, "unknown date") || strings.Contains(all, "--:--") {
		t.Fatal(all)
	}
	for _, size := range [][2]int{{140, 56}, {140, 40}, {100, 56}, {70, 20}} {
		m = resize(m, size[0], size[1])
		cols, rows := m.ArtworkSize()
		if cols != 0 && (cols != 16 || rows > 6) {
			t.Fatal("art dominates card", cols, rows)
		}
	}
}
