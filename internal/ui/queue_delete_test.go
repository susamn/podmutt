package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestQueueRemovalAndUndoHintsStayVisibleAtMinimumWidth(t *testing.T) {
	for _, width := range []int{30, 70, 140} {
		m := resize(New(Data{}), width, 20).WithFocus(Queue)
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "d/Del:remove") || !strings.Contains(view, "u:undo") {
			t.Fatal("core editing hints were clipped", width, view)
		}
	}
}
