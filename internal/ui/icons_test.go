package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestIconVocabularyModesAndSafeFallback(t *testing.T) {
	for _, mode := range []string{"", "unicode", "nerd", "plain", "invalid"} {
		m := New(Data{}).WithIcons(mode)
		for icon := ShowIcon; icon <= QueueIcon; icon++ {
			value := m.Icon(icon, "Readable label")
			if !strings.Contains(value, "Readable label") {
				t.Fatal("icon replaced text")
			}
			if mode == "plain" || mode == "invalid" {
				if value != "Readable label" {
					t.Fatal("text fallback failed")
				}
			} else {
				if ansi.StringWidth(value) != 16 {
					t.Fatal("non-cell-safe icon", mode, icon, value, ansi.StringWidth(value))
				}
			}
		}
		if m.Icon(Icon(255), "Unknown") != "Unknown" {
			t.Fatal("unknown icon did not fall back")
		}
	}
	if stateIcon("paused") != PausedIcon || stateIcon("failed") != FailedIcon || stateIcon("unknown") != StoppedIcon {
		t.Fatal("state vocabulary")
	}
}

func TestIconLayoutsPreserveLabelsSelectionAndWidths(t *testing.T) {
	for _, mode := range []string{"unicode", "nerd", "plain"} {
		for _, size := range [][2]int{{160, 44}, {100, 24}, {70, 20}, {30, 12}} {
			data := fixture()
			data.PodcastLabel = "Discover · Apple Podcasts"
			data.Player = Player{Status: "playing", Title: "Unsafe\x1b[2J title 界", HasCurrent: true}
			m := resize(New(data).WithIcons(mode), size[0], size[1]).Select(Episodes, data.Episodes[len(data.Episodes)-1].ID)
			selected, _ := m.Selected(Episodes)
			if selected.ID != data.Episodes[len(data.Episodes)-1].ID {
				t.Fatal("icons altered selection")
			}
			view := m.View().Content
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("icon clipping overflow", mode, size)
				}
			}
			if !strings.Contains(ansi.Strip(view), "[Pause] [Stop] [Next]") || strings.Contains(view, "\x1b[2J") {
				t.Fatal("labels lost or metadata injected escape", mode, size)
			}
			for _, label := range []string{"RSS/website · preview", "My subscriptions"} {
				m.data.PodcastLabel = label
				pane := m.pane(Podcasts, 40, 8)
				if !strings.Contains(pane, "Library") && !strings.Contains(pane, "RSS/website") {
					t.Fatal("show labels lost")
				}
			}
		}
	}
}
