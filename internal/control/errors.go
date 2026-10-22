package control

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"podmutt/internal/feedback"
)

// Keep the existing error-string field: older clients reject unknown JSON
// fields. They see human-readable text first, while new clients read a versioned
// category trailer, never matching English or echoing arbitrary legacy errors.
const errorTrailer = "\n[podmutt-error-v1:"

func encodeError(err error) string {
	message := feedback.For("Playback command", err)
	status := 0
	var fault *feedback.Fault
	if errors.As(err, &fault) {
		status = fault.Status
	}
	return fmt.Sprintf("%s%s%s:%d]", message.Text(), errorTrailer, message.Code, status)
}
func decodeError(text string) error {
	index := strings.LastIndex(text, errorTrailer)
	if index >= 0 && strings.HasSuffix(text, "]") {
		code, value, ok := strings.Cut(text[index+len(errorTrailer):len(text)-1], ":")
		status, err := strconv.Atoi(value)
		if ok && err == nil && feedback.ValidCode(feedback.Code(code)) && status >= 0 && status <= 599 && (feedback.Code(code) != feedback.HTTP || status >= 100) {
			return &feedback.Fault{Code: feedback.Code(code), Status: status}
		}
	}
	return feedback.New(feedback.Protocol, "The running instance returned an unsupported error response.")
}
