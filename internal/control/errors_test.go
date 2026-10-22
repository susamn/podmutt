package control

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"podmutt/internal/feedback"
	"podmutt/internal/playback"
)

func TestControlErrorsPreserveCategoriesWithoutChangingJSONShape(t *testing.T) {
	for _, err := range []error{playback.ErrNoEpisode, feedback.StatusError(503), errors.New("https://user:private-secret@example.test/?token=private-secret")} {
		encoded := encodeError(err)
		decoded := decodeError(encoded)
		if feedback.For("Playback command", decoded).Code != feedback.For("Playback command", err).Code || strings.Contains(encoded, "private-secret") {
			t.Fatal(encoded, decoded)
		}
		// Old strict decoders still accept the response. Their error starts with
		// readable copy; no additional JSON fields or protocol handshake is needed.
		data, marshalErr := json.Marshal(response{Error: encoded})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var legacy struct {
			State *Status `json:"state,omitempty"`
			Error string  `json:"error,omitempty"`
		}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&legacy); err != nil || !strings.HasPrefix(legacy.Error, "No current episode") && !strings.Contains(legacy.Error, "failed") && !strings.Contains(legacy.Error, "HTTP 503") {
			t.Fatal(string(data), err)
		}
	}
}

func TestLegacyAndMalformedErrorsNeverEchoUntrustedText(t *testing.T) {
	for _, wire := range []string{"private-secret legacy error", "bad\n[podmutt-error-v1:private-secret:0]", "bad\n[podmutt-error-v1:http:0]", "bad\n[podmutt-error-v1:http:999]", "bad\n[podmutt-error-v1:timeout:invalid]", "bad\n[podmutt-error-v1:timeout]"} {
		err := decodeError(wire)
		message := feedback.For("Playback command", err)
		if message.Code != feedback.Protocol || strings.Contains(message.Text(), "private-secret") {
			t.Fatal(message)
		}
	}
}
