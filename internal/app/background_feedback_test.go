package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"podmutt/internal/feed"
)

func TestBackgroundFailuresCannotReplaceForegroundRecovery(t *testing.T) {
	librarySession(t, func(s *session) {
		m := browserLibrary(t, s, &fixtureDirectory{err: context.DeadlineExceeded})
		finishLibrary(&m, m.searchDirectory("science"))
		foreground := m.notice
		updateLibrary(&m, latestQueued{err: errors.New("private-secret queue error")})
		updateLibrary(&m, libraryLoaded{version: m.version, err: errors.New("private-secret read error")})
		if m.notice != foreground || !strings.Contains(m.View().Content, "Apple Podcasts search timed out") || strings.Contains(m.View().Content, "private-secret") {
			t.Fatal("background replaced foreground recovery")
		}
		if !strings.Contains(m.backgroundFeedback().Text(), "Load library failed") {
			t.Fatal("background failure lost")
		}
		m.clearBackgroundFailure("Update latest episodes")
		if m.backgroundError.Summary == "" {
			t.Fatal("unrelated success cleared failure")
		}
		m.clearBackgroundFailure("Load library")
		if m.backgroundFeedback().Summary != "" {
			t.Fatal("recovery retained background error")
		}
		m.recordBackgroundFailure("Load library", context.Canceled)
		if m.backgroundError.Summary != "" {
			t.Fatal("cancellation became warning")
		}
		m.refreshErrors[0] = context.DeadlineExceeded
		m.recordBackgroundFailure("Update latest episodes", errors.New("queue failed"))
		if !strings.Contains(m.backgroundFeedback().Action, "Update latest episodes failed") {
			t.Fatal("aggregate dropped local failure")
		}
		updateLibrary(&m, latestQueued{})
		if m.backgroundError.Summary != "" {
			t.Fatal("successful queue update retained failure")
		}
		updateLibrary(&m, refreshUpdate{result: feed.Result{FeedID: 99}, ok: true})
		if len(m.refreshErrors) != 0 {
			t.Fatal("successful refresh retained cycle-start failure")
		}
	})
}
