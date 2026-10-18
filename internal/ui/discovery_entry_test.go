package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDiscoveryActionsReserveViewportWithoutBecomingItems(t *testing.T) {
	for _, size := range [][2]int{{160, 44}, {100, 24}, {70, 20}, {30, 12}} {
		data := fixture()
		data.PodcastHint = "f Search podcasts\nU RSS/website preview\nt Apple US top 25\x1b[2J"
		m := resize(New(data), size[0], size[1])
		m = m.Select(Podcasts, data.Podcasts[len(data.Podcasts)-1].ID)
		if len(m.Visible(Podcasts)) != len(data.Podcasts) {
			t.Fatal("actions became filterable/selectable subscriptions")
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("action width overflow", size)
			}
		}
		pane := ansi.Strip(m.pane(Podcasts, size[0], 8))
		if !strings.Contains(pane, data.Podcasts[len(data.Podcasts)-1].Title) || strings.Contains(pane, "\x1b[2J") {
			t.Fatal("hint hid selected row or injected escape", pane)
		}
	}
}
