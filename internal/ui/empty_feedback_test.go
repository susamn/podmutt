package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEmptyFilterDoesNotPretendTheLibraryOrQueueIsEmpty(t *testing.T) {
	m := resize(New(fixture()), 140, 40)
	for _, test := range []struct {
		panel Panel
		want  string
	}{{Podcasts, "No matching shows"}, {Episodes, "No matching episodes"}, {Queue, "No matching queue episodes"}} {
		filtered := m.WithFocus(test.panel)
		filtered = press(filtered, "/", "z", "z", "z", "enter")
		if !strings.Contains(ansi.Strip(filtered.View().Content), test.want) {
			t.Fatal(test.panel, filtered.View().Content)
		}
	}
	empty := resize(New(Data{}), 140, 40)
	if text := ansi.Strip(empty.View().Content); !strings.Contains(text, "No shows") || !strings.Contains(text, "No episodes for this selection") {
		t.Fatal(text)
	}
}
