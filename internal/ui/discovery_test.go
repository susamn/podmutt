package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDiscoveryLabelsHintsAndFilterResetFitAllLayouts(t *testing.T) {
	for _, size := range [][2]int{{160, 36}, {80, 24}, {30, 12}} {
		m := resize(New(Data{PodcastLabel: "Discover · Apple Podcasts · US top 25\x1b[2J", ContextHint: "b My subscriptions · f search", Podcasts: []Item{{ID: "one", Title: "One"}, {ID: "two", Title: "Two"}}}), size[0], size[1])
		m = press(m, "/")
		m = press(m, "O")
		m = press(m, "n")
		m = press(m, "e")
		m = press(m, "enter")
		if len(m.Visible(Podcasts)) != 1 {
			t.Fatal("filter fixture")
		}
		m = m.ClearFilter(Podcasts).ClearFilter(Details)
		if len(m.Visible(Podcasts)) != 2 {
			t.Fatal("mode switch left hidden rows")
		}
		for _, line := range strings.Split(m.View().Content, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatal("layout overflow")
			}
		}
		if strings.Count(m.View().Content, "\n") >= size[1] {
			t.Fatal("vertical overflow")
		}
	}
}
