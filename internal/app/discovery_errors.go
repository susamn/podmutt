package app

import (
	"context"
	"errors"

	"podmutt/internal/feedback"
)

// Never render arbitrary transport/parser errors, which may contain URLs and
// credentials. Keep recovery specific to the operation actually being retried.
func discoveryFailure(operation string, err error, retry string) string {
	if err == nil || errors.Is(err, context.Canceled) {
		return ""
	}
	return feedback.Retry(feedback.For(operation, err), retry).Text()
}
