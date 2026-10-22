package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInvalidURLStaysEditableWithVisibleFeedback(t *testing.T) {
	librarySession(t, func(s *session) {
		directory := &fixtureDirectory{}
		m := browserLibrary(t, s, directory)
		updateLibrary(&m, tea.WindowSizeMsg{Width: 30, Height: 12})
		m.openURL()
		updateLibrary(&m, tea.PasteMsg{Content: "https://user:private-secret@example.test/rss"})
		if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || !m.browse.searching || directory.calls != 0 {
			t.Fatal("invalid URL dispatched")
		}
		if strings.Contains(m.notice, "private-secret") || !strings.Contains(m.View().Content, "HTTP(S)") || !strings.Contains(m.View().Content, "Esc cancel") {
			t.Fatal("feedback hidden", m.notice, m.View().Content)
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		libraryKey(&m, 'N')
		m.input = "invalid"
		if cmd := updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil || !m.adding || !strings.Contains(m.notice, "HTTP(S)") {
			t.Fatal("invalid Add URL closed input")
		}
	})
}
