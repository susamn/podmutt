// Package feedback translates classified errors at user-facing boundaries.
// Never render arbitrary error strings: transport/parser/OS errors can contain
// credentials, request URLs or configuration contents. Causes remain unwrap-able.
package feedback

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

type Code string

const (
	Partial         Code = "partial"
	Input           Code = "input"
	Unknown         Code = "unknown"
	Canceled        Code = "canceled"
	Timeout         Code = "timeout"
	Network         Code = "network"
	InvalidResponse Code = "invalid_response"
	InvalidURL      Code = "invalid_url"
	RateLimited     Code = "rate_limited"
	Credentials     Code = "credentials"
	Authentication  Code = "authentication"
	HTTP            Code = "http"
	Configuration   Code = "configuration"
	NewerDatabase   Code = "newer_database"
	Database        Code = "database"
	Profile         Code = "profile"
	AlreadyRunning  Code = "already_running"
	NotRunning      Code = "not_running"
	SessionClosed   Code = "session_closed"
	Protocol        Code = "protocol"
	NoEpisode       Code = "no_episode"
	QueueChanged    Code = "queue_changed"
	NoUndo          Code = "no_undo"
	SeekUnavailable Code = "seek_unavailable"
	PlayerMissing   Code = "player_missing"
	PlayerFailure   Code = "player_failure"
	Import          Code = "import"
)

func ValidCode(code Code) bool {
	switch code {
	case Partial, Input, Unknown, Canceled, Timeout, Network, InvalidResponse, InvalidURL, RateLimited, Credentials, Authentication, HTTP, Configuration, NewerDatabase, Database, Profile, AlreadyRunning, NotRunning, Protocol, SessionClosed, NoEpisode, QueueChanged, NoUndo, SeekUnavailable, PlayerMissing, PlayerFailure, Import:
		return true
	}
	return false
}

type Fault struct {
	Code   Code
	Safe   string // fixed, application-authored explanation; NEVER raw err.Error()
	Status int
	Cause  error
}

func (e *Fault) Error() string {
	if e.Safe != "" {
		return e.Safe
	}
	return string(e.Code)
}
func (e *Fault) Unwrap() error         { return e.Cause }
func New(code Code, safe string) error { return &Fault{Code: code, Safe: safe} }
func Wrap(code Code, err error) error  { return &Fault{Code: code, Cause: err} }
func StatusError(status int) error {
	code := HTTP
	if status == 429 {
		code = RateLimited
	}
	return &Fault{Code: code, Status: status, Safe: fmt.Sprintf("HTTP %d", status)}
}

// Message contains only safe presentation data. Code is also usable in the
// private control protocol without changing successful status JSON or exit codes.
type Message struct {
	Code     Code
	Scope    string
	Severity string
	Summary  string
	Action   string
}

func (m Message) Text() string { return strings.TrimSpace(m.Summary + " " + m.Action) }
func For(operation string, err error) Message {
	m := Message{Scope: operation, Code: Unknown, Severity: "error", Summary: operation + " failed.", Action: "Try the action again. If it keeps failing, report it at github.com/susamn/podmutt/issues with your podmutt version."}
	if err == nil {
		return Message{Severity: "info"}
	}
	var fault *Fault
	var network net.Error
	switch {
	case errors.As(err, &fault) && fault.Code == Partial:
		m.Code = Partial
	case errors.Is(err, context.Canceled):
		m.Code = Canceled
	case errors.Is(err, context.DeadlineExceeded):
		m.Code = Timeout
	case errors.As(err, &network) && network.Timeout():
		m.Code = Timeout
	case errors.As(err, &fault):
		m.Code = fault.Code
	case os.IsPermission(err):
		m.Code = Profile
	case errors.Is(err, syscall.ENOSPC):
		m.Code = Database
	default:
		if errors.As(err, &network) {
			m.Code = Network
		}
	}
	switch m.Code {
	case Partial:
		m.Severity = "warning"
		m.Summary = fault.Safe
		m.Action = "Open Library and press R to refresh feeds and retry queue updates."
	case Canceled:
		m.Severity = "info"
		m.Summary = operation + " canceled."
		m.Action = ""
	case Timeout:
		m.Summary = operation + " timed out."
		m.Action = "Try again. A timeout does not necessarily mean you are offline."
	case Network:
		m.Summary = "Could not connect for " + operation + "."
		m.Action = "Check your connection and try again."
	case InvalidResponse:
		m.Summary = operation + " returned unreadable or unsupported data."
		m.Action = "Check the source or try again later."
	case Input:
		m.Summary = fault.Safe
		m.Action = "Review your input and submit it again."
	case InvalidURL:
		m.Summary = "Use an HTTP(S) URL without credentials."
		m.Action = "Enter an http:// or https:// podcast feed or website URL without a username or password."
	case RateLimited:
		m.Summary = "Too many requests for " + operation + "."
		m.Action = "Wait a moment, then try again."
	case Credentials:
		m.Summary = "Podcast Index requires your own PODMUTT_PODCAST_INDEX_KEY and PODMUTT_PODCAST_INDEX_SECRET."
		m.Action = "Set both before starting podmutt, or use Apple Podcasts or an RSS URL without a key."
	case Authentication:
		m.Summary = operation + " was not authorized."
		m.Action = "Check your API access and system clock, then try again."
	case HTTP:
		m.Summary = fmt.Sprintf("%s returned HTTP %d.", operation, fault.Status)
		switch fault.Status {
		case 401, 403:
			m.Action = "The source denied access. Check your API access or try a public source."
		case 404, 410:
			m.Action = "The source is unavailable at this address. Check the URL or try again later."
		default:
			m.Action = "The source could not complete the request. Try again later."
		}
	case NewerDatabase:
		m.Summary = "This profile's database was upgraded by a newer version of podmutt."
		m.Action = "Upgrade podmutt or run the newer binary used previously. Do not delete the database or change its schema version. " + fault.Safe
	case Database:
		m.Summary = "Could not read or save the local database."
		m.Action = "Check free disk space and profile permissions. Keep the database intact; do not reset its schema version."
	case Configuration:
		m.Summary = "Check config.toml: " + fault.Safe
		m.Action = "Correct the setting and try again."
	case Profile:
		m.Summary = "Could not access the private podmutt profile."
		m.Action = "Check that its files belong to your user and the state/runtime directories are private (0700). Do not run podmutt as root."
	case AlreadyRunning:
		m.Summary = "podmutt is already running for this profile."
		m.Action = "Use the existing window or close it before starting another instance."
	case NotRunning:
		m.Summary = "podmutt is not running for this profile."
		m.Action = "Start podmutt in a terminal, then retry this command."
	case SessionClosed:
		m.Summary = "The playback session has closed."
		m.Action = "Restart podmutt before trying this action again."
	case Protocol:
		m.Summary = "Could not communicate with the running podmutt instance."
		m.Action = "Check that the CLI and running app use the same version; restart the app and retry."
	case NoEpisode:
		m.Summary = "No current episode."
		m.Action = "Press Enter on an episode to play it."
	case QueueChanged:
		m.Summary = "The queue changed before this edit finished."
		m.Action = "Review Queue and try again."
	case NoUndo:
		m.Severity = "info"
		m.Summary = "Nothing to undo in Queue."
		m.Action = ""
	case SeekUnavailable:
		m.Summary = "Seeking is not available yet."
		m.Action = "Load an episode and wait until it is ready before seeking."
	case PlayerMissing:
		m.Summary = "mpv is not installed or could not be found."
		m.Action = "Install mpv with your package manager (macOS: brew install mpv), or correct mpv_path in config.toml."
	case PlayerFailure:
		m.Summary = operation + " failed."
		m.Action = "Check the episode source and mpv installation, then try playback again."
	case Import:
		m.Summary = "Could not import the OPML file."
		if fault.Safe != "" {
			m.Summary = fault.Safe
		}
		m.Action = "Check the OPML export, failing feed URLs, profile permissions and free disk space, then retry. Existing subscriptions are kept."
	}
	return m
}

// Retry adds an operation-specific, tested key/invocation only to retryable
// failures, not malformed input, credentials/setup or local persistence errors.
func Retry(m Message, action string) Message {
	switch m.Code {
	case Network:
		m.Action = "Check your connection. " + action
	case RateLimited:
		m.Action = "Wait a moment before retrying. " + action
	case Unknown, Timeout, InvalidResponse, HTTP:
		m.Action = action
	}
	return m
}

// PublicLabel bounds and sanitizes metadata used in feedback/import output.
// Do not echo links (even with credentials or sensitive query parameters).
func PublicLabel(text string) string {
	text = ansi.Strip(text)
	words := strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text))
	for i, word := range words {
		if strings.Contains(word, "://") {
			words[i] = "[link]"
		}
	}
	return ansi.Truncate(strings.Join(words, " "), 100, "…")
}
