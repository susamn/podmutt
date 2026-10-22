package app

import (
	"context"
	"errors"
	"net"

	"podmutt/internal/discovery"
)

// discoveryFailure presents only application-authored text, not arbitrary
// transport/parser errors which can contain request URLs and credentials.
// The caller supplies the operation and a retry action that actually works.
func discoveryFailure(operation string, err error, retry string) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	var network net.Error
	summary := operation + " failed."
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		summary = operation + " timed out."
	case errors.As(err, &network) && network.Timeout():
		summary = operation + " timed out."
	case errors.Is(err, discovery.ErrIndexCredentials):
		return "Podcast Index requires your own PODMUTT_PODCAST_INDEX_KEY and PODMUTT_PODCAST_INDEX_SECRET. Configure them before starting podmutt, or press f and Tab to choose another search source."
	case errors.Is(err, discovery.ErrQuery):
		return "Enter a podcast name between 1 and 128 characters. Press f to edit the search."
	case errors.Is(err, discovery.ErrRateLimited):
		summary = "Too many requests for " + operation + ". Wait a moment before trying again."
	case errors.Is(err, discovery.ErrUnavailable):
		summary = "Could not reach the source for " + operation + ". Check your connection."
	case errors.As(err, &network):
		summary = "Could not reach the source for " + operation + ". Check your connection."
	}
	return summary + " " + retry
}
