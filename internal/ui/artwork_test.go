package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestArtworkColumnKeepsDetailsScrollableAndPlayerBounded(t *testing.T) {
	m := resize(New(Data{Details: strings.Repeat("Very long details 界 ", 2000), Player: Player{Title: "Current audio"}}), 140, 44)
	cols, rows := m.ArtworkSize()
	if cols < 12 || rows < 1 {
		t.Fatal(cols, rows)
	}
	art := make([]string, rows)
	for i := range art {
		art[i] = strings.Repeat("@", cols)
	}
	m = m.WithArtwork(cols, art)
	view := m.View().Content
	if lipgloss.Width(view) != 140 || lipgloss.Height(view) != 44 || !strings.Contains(view, "@@@@") {
		t.Fatal("art enlarged layout")
	}
	m.scrollDetails(100)
	if m.detailScroll == 0 {
		t.Fatal("details didn't scroll")
	}
	if !strings.Contains(m.View().Content, "Player") {
		t.Fatal("player displaced")
	}
	m = resize(m, 80, 24)
	if c, r := m.ArtworkSize(); c != 0 || r != 0 {
		t.Fatal("compact art sized")
	}
	if strings.Contains(ansi.Strip(m.View().Content), "@@@@") {
		t.Fatal("art persisted in compact pane")
	}
	m = resize(m, 140, 44).WithDialog(&Dialog{Title: "modal"})
	if c, _ := m.ArtworkSize(); c != 0 {
		t.Fatal("art overlay leaks")
	}
}
