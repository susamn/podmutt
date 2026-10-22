package app

import (
	"strings"
	"testing"

	"podmutt/internal/feed"
)

func TestPreviewAttributionDoesNotEchoURLCredentialsOrQueryTokens(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		m.browse.directURL = true
		m.browse.preview = &feed.Resolved{URL: "https://user:private-secret@example.test/rss?token=private-secret#private-secret"}
		for _, directory := range []bool{false, true} {
			if directory {
				m.browse.source = "https://user:private-secret@example.test/show?token=private-secret#private-secret"
			}
			m.renderPreviewDetails()
			view := m.View().Content
			if strings.Contains(view, "private-secret") || !strings.Contains(view, "https://example.test/") {
				t.Fatal(view)
			}
		}
	})
}
