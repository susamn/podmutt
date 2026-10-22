package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEpisodeRemovalConfirmationKeepsDecisionAndDetailsVisible(t *testing.T) {
	for _, size := range [][2]int{{30, 12}, {70, 20}, {140, 40}} {
		dialog := &Dialog{Title: "Stop & remove? y/n", Body: strings.Repeat("Episode 界", 100) + "\n" + strings.Repeat("Show 界", 100) + "\nplaying · 0:42/48:00\nProgress/history kept.", TruncateBody: true, Hint: "y remove · n/Esc cancel"}
		m := resize(New(Data{}), size[0], size[1]).WithDialog(dialog)
		view := ansi.Strip(m.View().Content)
		for _, part := range []string{"y/n", "Episode", "Show", "playing", "Progress/history kept.", "y remove", "n/Esc cancel"} {
			if !strings.Contains(view, part) {
				t.Fatal(size, part, view)
			}
		}
		rows := strings.Split(view, "\n")
		if len(rows) != size[1] {
			t.Fatal("confirmation resized frame", size)
		}
		for _, row := range rows {
			if ansi.StringWidth(row) > size[0] {
				t.Fatal("overflow", size, row)
			}
		}
	}
	// Even extra application body rows cannot displace the decision keys.
	m := resize(New(Data{}), 30, 12).WithDialog(&Dialog{Title: "Remove? y/n", Body: strings.Repeat("Detail row\n", 30), TruncateBody: true, Hint: "y remove · n/Esc cancel"})
	if !strings.Contains(ansi.Strip(m.View().Content), "n/Esc cancel") {
		t.Fatal("decision keys clipped")
	}
}
