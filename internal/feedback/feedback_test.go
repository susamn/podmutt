package feedback

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
)

type networkError struct{ timeout bool }

func (e networkError) Error() string   { return "https://user:secret@example.test/?token=secret" }
func (e networkError) Timeout() bool   { return e.timeout }
func (e networkError) Temporary() bool { return true }

var _ net.Error = networkError{}

func TestEveryCategoryHasSafeActionableCopy(t *testing.T) {
	codes := []Code{Partial, Input, Unknown, Canceled, Timeout, Network, InvalidResponse, InvalidURL, RateLimited, Credentials, Authentication, HTTP, Configuration, NewerDatabase, Database, Profile, AlreadyRunning, NotRunning, Protocol, SessionClosed, NoEpisode, QueueChanged, NoUndo, SeekUnavailable, PlayerMissing, PlayerFailure, Import}
	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			err := &Fault{Code: code, Safe: "Application-authored explanation.", Status: 503, Cause: errors.New("https://user:secret@example.test/?token=secret\x1b[2J TOML-secret")}
			message := For("Test operation", fmt.Errorf("outer: %w", err))
			if !ValidCode(code) || message.Code != code || message.Scope != "Test operation" || message.Summary == "" || message.Severity == "" {
				t.Fatal(message)
			}
			if strings.Contains(message.Text(), "secret") || strings.Contains(message.Text(), "\x1b") {
				t.Fatal("unsafe cause rendered", message)
			}
			if !errors.Is(err, err.Cause) {
				t.Fatal("cause not preserved")
			}
			if err.Error() != "Application-authored explanation." {
				t.Fatal("diagnostic leaked cause")
			}
		})
	}
	if ValidCode(Code("secret")) {
		t.Fatal("unknown code accepted")
	}
	if got := For("Test", nil); got.Text() != "" || got.Severity != "info" {
		t.Fatal(got)
	}
	if err := Wrap(Network, errors.New("secret")); err.Error() != "network" {
		t.Fatal("unsafe diagnostic", err)
	}
}

func TestClassificationPreservesCancellationTimeoutAndOSErrors(t *testing.T) {
	cases := []struct {
		err  error
		code Code
	}{
		{fmt.Errorf("outer: %w", context.Canceled), Canceled},
		{Wrap(Network, context.DeadlineExceeded), Timeout},
		{Wrap(Network, networkError{true}), Timeout},
		{networkError{false}, Network},
		{os.ErrPermission, Profile},
		{syscall.ENOSPC, Database},
		{errors.New("arbitrary secret parser contents"), Unknown},
	}
	for _, tc := range cases {
		m := For("Search", tc.err)
		if m.Code != tc.code || strings.Contains(m.Text(), "secret") {
			t.Fatal(m)
		}
	}
	partial := &Fault{Code: Partial, Safe: "Subscription change saved.", Cause: context.DeadlineExceeded}
	if got := For("Library update", partial); got.Code != Partial || !strings.Contains(got.Text(), "saved") {
		t.Fatal("partial persistence misreported", got)
	}
}

func TestHTTPAndRetryActions(t *testing.T) {
	for _, status := range []int{401, 403, 404, 410, 429, 503} {
		err := StatusError(status)
		got := For("Feed preview", err)
		if got.Summary == "" || got.Action == "" {
			t.Fatal(got)
		}
		if status == 429 && got.Code != RateLimited {
			t.Fatal(got)
		}
	}
	for _, code := range []Code{Unknown, Timeout, Network, InvalidResponse, HTTP, RateLimited} {
		m := Retry(For("Search", &Fault{Code: code, Status: 500}), "Press R to retry.")
		want := "Press R to retry."
		if code == RateLimited {
			want = "Wait a moment before retrying. " + want
		}
		if code == Network {
			want = "Check your connection. " + want
		}
		if m.Action != want {
			t.Fatal(m)
		}
	}
	m := For("Preview", New(InvalidURL, "static"))
	if Retry(m, "Press R").Action != m.Action {
		t.Fatal("invalid URL suggested blind retry")
	}
}

func TestLabelsDoNotExposeLinksOrTerminalControls(t *testing.T) {
	for _, text := range []string{"\x1b[2JShow\nname https://user:secret@example.test/rss?token=secret", "\x1b]8;;https://secret\x07Show\x1b]8;;\x07"} {
		got := PublicLabel(text)
		if strings.Contains(got, "secret") || strings.Contains(got, "\x1b") || strings.Contains(got, "\n") {
			t.Fatal(got)
		}
	}
	if PublicLabel(strings.Repeat("界", 200)) == strings.Repeat("界", 200) {
		t.Fatal("unbounded metadata")
	}
}
