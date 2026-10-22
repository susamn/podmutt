package app

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"podmutt/internal/feed"
	"podmutt/internal/feedback"
)

func TestForegroundFailureSurvivesBackgroundUpdatesAndClearsOnRetry(t *testing.T) {
	librarySession(t, func(s *session) {
		d := &fixtureDirectory{err: context.DeadlineExceeded}
		m := browserLibrary(t, s, d)
		finishLibrary(&m, m.searchDirectory("science"))
		original := m.notice
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: 99}, ok: true})
		if m.notice != original || !strings.Contains(m.View().Content, "Error · ! details") {
			t.Fatal("background work hid foreground failure")
		}
		libraryKey(&m, '!')
		if !m.Model.Busy() {
			t.Fatal("full explanation unavailable")
		}
		updateLibrary(&m, tea.KeyPressMsg{Code: tea.KeyEscape})
		d.err = nil
		finishLibrary(&m, m.searchDirectory("science"))
		if m.browse.errorText != "" || strings.Contains(m.View().Content, "Error · ! details") {
			t.Fatal("retry retained old failure")
		}
	})
}

func TestRefreshFeedbackNamesAllFailuresAndKeepsCorrectRetryContext(t *testing.T) {
	librarySession(t, func(s *session) {
		a, b := seededLibrary(t, s)
		m, err := newLibrary(s, feed.New(s.store, nil, 0))
		if err != nil {
			t.Fatal(err)
		}
		m.refreshErrors[a] = context.DeadlineExceeded
		m.refreshErrors[b] = feedback.StatusError(404)
		message := m.refreshFeedback()
		for _, want := range []string{"Alpha show", "Beta show", "Cached episodes", "HTTP 404"} {
			if !strings.Contains(message.Action, want) {
				t.Fatal(message)
			}
		}
		m.browse.active = true
		if !strings.Contains(m.refreshFeedback().Action, "Press b") {
			t.Fatal("wrong view retry hint")
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: a, Error: context.Canceled}, ok: true})
		if m.refreshErrors[a] != context.DeadlineExceeded {
			t.Fatal("cancel replaced useful failure")
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: 99}, ok: true})
		if len(m.refreshErrors) != 2 {
			t.Fatal("unrelated success cleared failures")
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: a}, ok: true})
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: b}, ok: true})
		if m.refreshFeedback().Summary != "" {
			t.Fatal("successful refresh retained warning")
		}
	})
}
