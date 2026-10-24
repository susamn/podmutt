package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestPlaybackIdentityAndDialogSurviveLibraryReload(t *testing.T) {
	m := New(Data{Queue: []Item{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}}).Select(Queue, "b")
	m = m.WithPlayback([]Item{{ID: "b", Title: "Beta"}, {ID: "a", Title: "Alpha"}}, Player{Status: "playing"})
	if item, _ := m.Selected(Queue); item.ID != "b" {
		t.Fatal("identity lost")
	}
	m = m.WithDialog(&Dialog{Title: "Confirm\x1b[2J", Body: "Remove\nBeta?", Hint: "y confirm"})
	m = m.WithLibrary(Data{Episodes: []Item{{ID: "c", Title: "New"}}})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 70, Height: 20})
	m = next.(Model)
	if !m.Busy() || !strings.Contains(ansi.Strip(m.View().Content), "y confirm") {
		t.Fatal("dialog lost on reload")
	}
	if items := m.Visible(Queue); len(items) != 2 {
		t.Fatal("queue replaced by library")
	}
	if len(m.Visible(Details)) != 0 {
		t.Fatal("details treated as list")
	}
	m = m.WithDialog(nil)
	if m.Busy() {
		t.Fatal("dialog stuck")
	}
}
