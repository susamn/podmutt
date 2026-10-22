package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"podmutt/internal/discovery"
	"podmutt/internal/feedback"
)

func TestDirectoryFeedbackCancellationTimeoutAndStaleReplies(t *testing.T) {
	librarySession(t, func(s *session) {
		for _, err := range []error{context.DeadlineExceeded, discovery.ErrUnavailable, discovery.ErrRateLimited, errors.New("https://user:private-secret@example.test?token=private-secret")} {
			m := browserLibrary(t, s, &fixtureDirectory{err: err})
			finishLibrary(&m, m.searchDirectory("science"))
			want := feedback.Retry(feedback.For("Apple Podcasts search", err), "Press R to retry, f for another search, or b for your saved Library.").Text()
			if m.notice != want || m.browse.errorText != want || strings.Contains(m.notice, "private-secret") {
				t.Fatal(m.notice)
			}
		}
		m := browserLibrary(t, s, &fixtureDirectory{err: context.Canceled})
		finishLibrary(&m, m.searchDirectory("cancel"))
		if m.browse.loading || m.browse.errorText != "" || m.notice != "" {
			t.Fatal("cancellation shown as failure", m.notice)
		}
		stale := m.searchDirectory("old")
		m.cancelDirectory()
		m.notice = "new action"
		finishLibrary(&m, stale)
		if m.notice != "new action" {
			t.Fatal("stale cancellation overwrote new action")
		}
	})
}

func TestPreviewFeedbackUsesPreviewOperationAndIgnoresCancellation(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{})
		m.browse.shows = []discovery.Show{{Title: "Show", FeedURL: "https://example.test/rss"}}
		m.renderLibrary()
		msg := previewLoaded{generation: m.browse.previewGeneration, key: "https://example.test/rss", err: context.DeadlineExceeded}
		m.applyPreview(msg)
		if !strings.Contains(m.notice, "Feed preview timed out") || !strings.Contains(m.notice, "R to retry this feed preview") || strings.Contains(m.notice, "context deadline") {
			t.Fatal(m.notice)
		}
		msg.err = context.Canceled
		m.applyPreview(msg)
		if m.browse.errorText != "" || m.notice != "" {
			t.Fatal("canceled preview shown as failure")
		}
		m.browse.active = false
		m.applyDirectory(directoryLoaded{generation: m.browse.generation, local: true, err: context.DeadlineExceeded})
		if !strings.Contains(m.notice, "Library search timed out") || !strings.Contains(m.notice, "Press f") {
			t.Fatal(m.notice)
		}
	})
}
