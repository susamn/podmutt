package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestChapterPickerCompactPagingAndEscOwnership(t *testing.T) {
	m := New(Data{}).WithFocus(Episodes)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 12})
	m = updated.(Model)
	rows := make([]ChapterRow, 30)
	for i := range rows {
		rows[i] = ChapterRow{Time: fmt.Sprintf("%d:00", i), Title: fmt.Sprintf("Part %d", i)}
	}
	m = m.WithChapters(&ChapterPicker{Episode: "An episode", Show: "A show", Rows: rows, Selected: 0})
	if !strings.Contains(m.View().Content, "Part 0") || m.ChapterSelection() != 0 {
		t.Fatal("compact picker missing")
	}
	m = m.MoveChapter(m.ChapterPageSize())
	if m.ChapterSelection() < 1 || !strings.Contains(m.View().Content, fmt.Sprintf("Part %d", m.ChapterSelection())) {
		t.Fatal("picker paging hid selected row")
	}
	m = m.MoveChapter(100)
	if m.ChapterSelection() != 29 || !strings.Contains(m.View().Content, "Part 29") {
		t.Fatal("last chapter inaccessible")
	}
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if updated.(Model).ChapterSelection() != 29 {
		t.Fatal("UI key leaked behind picker")
	}
	m = m.WithChapters(nil)
	if m.ChapterSelection() != -1 || m.Focus() != Episodes {
		t.Fatal("closing picker lost origin")
	}
}
