package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestScrollableURLDialogRetainsFooterAndShowsLaterURLs(t *testing.T) {
	body := "Feed\n  RSS: https://example.test/" + strings.Repeat("path/", 14) + "\n\nEpisode\n  Audio: https://example.test/audio\n  Chapters: https://example.test/chapters"
	m := resize(New(Data{}), 42, 16)
	dialog := &Dialog{Title: "Current playback · URLs", Body: body, Hint: "j/k scroll · Esc close", Scrollable: true, URLLabels: true}
	first := ansi.Strip(m.WithDialog(dialog).View().Content)
	if !strings.Contains(first, "RSS:") || !strings.Contains(first, "Esc close") {
		t.Fatal("first page omitted feed or close hint", first)
	}
	dialog.Scroll = 100
	last := ansi.Strip(m.WithDialog(dialog).View().Content)
	if !strings.Contains(last, "Chapters:") || !strings.Contains(last, "https://example.test/chapters") || !strings.Contains(last, "Esc close") {
		t.Fatal("later URL inaccessible or footer clipped", last)
	}
}

func TestURLLabelsUseThemeColorsBoldUnderlineWithoutStylingAddresses(t *testing.T) {
	m := New(Data{})
	pal := m.Theme()
	section := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color(string(pal.Accent)))
	label := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color(string(pal.Cyan)))
	for _, test := range []struct{ line, styled string }{
		{"Feed", section.Render("Feed")},
		{"Episode · Playing", section.Render("Episode") + " · Playing"},
		{"  RSS: https://example.test/RSS:literal", "  " + label.Render("RSS:") + " https://example.test/RSS:literal"},
		{"  Audio: https://example.test/audio", "  " + label.Render("Audio:") + " https://example.test/audio"},
		{"  Artwork: https://example.test/cover", "  " + label.Render("Artwork:") + " https://example.test/cover"},
		{"  Chapters: https://example.test/chapters", "  " + label.Render("Chapters:") + " https://example.test/chapters"},
		{"  Unknown: https://example.test", "  Unknown: https://example.test"},
	} {
		if got := urlLabelRow(test.line, pal); got != test.styled {
			t.Fatalf("styled label %q: got %q, want %q", test.line, got, test.styled)
		}
	}

	// Escape sequences embedded in remote titles/URLs must be stripped before
	// theme styling is applied; they must not create arbitrary terminal styles.
	dialog := &Dialog{Title: "URLs", URLLabels: true, Body: "Feed\n  RSS: https://example.test/\x1b[31mred\x1b[0m"}
	view := ansi.Strip(m.renderDialogModal(dialog, 70, 10))
	if !strings.Contains(view, "RSS: https://example.test/red") || strings.Contains(view, "\x1b") {
		t.Fatal("unsafe URL rendering", view)
	}
}
