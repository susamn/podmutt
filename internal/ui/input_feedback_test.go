package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCompactInputReservesSpaceForValueAndRecoveryKeys(t *testing.T) {
	for _, size := range [][2]int{{30, 12}, {70, 20}, {140, 40}} {
		m := resize(New(Data{}), size[0], size[1]).WithDialog(&Dialog{Title: "Search", Body: strings.Repeat("Long explanation that must not hide input. ", 40), InputLabel: "Query or URL", InputValue: "test", Hint: "Esc cancel · Enter submit"})
		view := ansi.Strip(m.View().Content)
		for _, want := range []string{"Query or URL", "test", "Esc cancel"} {
			if !strings.Contains(view, want) {
				t.Fatal(size, want, view)
			}
		}
		if len(strings.Split(view, "\n")) != size[1] {
			t.Fatal("frame grew")
		}
		for _, row := range strings.Split(view, "\n") {
			if ansi.StringWidth(row) > size[0] {
				t.Fatal("overflow", size, row)
			}
		}
	}
}
