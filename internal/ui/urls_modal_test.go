package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestScrollableURLDialogRetainsFooterAndShowsLaterURLs(t *testing.T) {
	body := "Feed\n  RSS: https://example.test/" + strings.Repeat("path/", 14) + "\n\nEpisode\n  Audio: https://example.test/audio\n  Chapters: https://example.test/chapters"
	m := resize(New(Data{}), 42, 16)
	dialog := &Dialog{Title: "Current playback · URLs", Body: body, Hint: "j/k scroll · Esc close", Scrollable: true}
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
