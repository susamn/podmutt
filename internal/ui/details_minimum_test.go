package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInspectedIdentitySurvivesMinimumLayoutWithMediaWarnings(t *testing.T) {
	for _, playbackError := range []string{"", "Playback failed"} {
		player := Player{Status: "paused", HasCurrent: true, Title: "Current audio", Warning: "Media backend unavailable; cached browsing and playback remain usable", Error: playbackError}
		card := EpisodeDetails{Key: "selected", Title: "Inspected episode", Show: "RSS show", Date: "2026-04-12", Duration: "1:15:00", State: "Not started", Inspecting: true, Description: strings.Repeat("Readable description\n", 50)}
		m := resize(New(Data{Player: player}), 30, 12).WithFocus(Details).WithEpisodeDetails(card)
		_, height := m.detailsSize()
		if height != 4 {
			t.Fatal("fixture must constrain Details to two interior rows", height)
		}
		for _, key := range []string{"g", "G", "ctrl+u", "ctrl+d"} {
			m = press(m, key)
			view := ansi.Strip(m.View().Content)
			for _, text := range []string{"Inspected episode", "Current audio", "[Play] [Stop] [Next]", "Media"} {
				if !strings.Contains(view, text) {
					t.Fatal("minimum layout lost identity/control/warning", text, view)
				}
			}
			if len(strings.Split(view, "\n")) != 12 {
				t.Fatal("layout grew")
			}
			for _, row := range strings.Split(view, "\n") {
				if ansi.StringWidth(row) > 30 {
					t.Fatal("layout overflow", row)
				}
			}
		}
		if m.detailScroll == 0 {
			t.Fatal("identity consumed the only reading row")
		}
	}
}
