package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"podmutt/internal/discovery"
)

type discoveryNetworkError struct{ timeout bool }

func (e discoveryNetworkError) Error() string {
	return "https://user:private-secret@example.test/rss?token=private-secret"
}
func (e discoveryNetworkError) Timeout() bool   { return e.timeout }
func (e discoveryNetworkError) Temporary() bool { return true }

func TestDiscoveryFailureClassifiesWithoutEchoingRawErrors(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		want  string
		retry bool
	}{
		{"success", nil, "", false},
		{"canceled", context.Canceled, "", false},
		{"wrapped cancellation", fmt.Errorf("transport: %w", context.Canceled), "", false},
		{"deadline", context.DeadlineExceeded, "Apple Podcasts search timed out.", true},
		{"transport timeout", discoveryNetworkError{true}, "Apple Podcasts search timed out.", true},
		{"connection", discoveryNetworkError{false}, "Check your connection.", true},
		{"directory unavailable", fmt.Errorf("transport: %w", discovery.ErrUnavailable), "Could not connect", true},
		{"rate limit", discovery.ErrRateLimited, "Wait a moment", true},
		{"query", discovery.ErrQuery, "between 1 and 128 characters", false},
		{"Index setup", discovery.ErrIndexCredentials, "PODMUTT_PODCAST_INDEX_KEY", false},
		{"unknown", errors.New("private-secret XML/config/HTTP response"), "Apple Podcasts search failed.", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := discoveryFailure("Apple Podcasts search", tc.err, "Press R to retry.")
			if tc.want == "" {
				if got != "" {
					t.Fatal(got)
				}
				return
			}
			if !strings.Contains(got, tc.want) || strings.Contains(got, "private-secret") || strings.Contains(got, "context deadline") {
				t.Fatal(got)
			}
			if strings.Contains(got, "Press R to retry.") != tc.retry {
				t.Fatal("incorrect recovery action", got)
			}
		})
	}
}

func TestDiscoveryFailureUsesTheActualOperationAndRecovery(t *testing.T) {
	for operation, retry := range map[string]string{
		"Feed preview":         "Press R to retry this feed preview.",
		"Podcast Index search": "Press R to retry, or f for another search.",
		"Library search":       "Press f to search your saved Library again.",
	} {
		got := discoveryFailure(operation, context.DeadlineExceeded, retry)
		if got != operation+" timed out. "+retry {
			t.Fatal(got)
		}
	}
}
