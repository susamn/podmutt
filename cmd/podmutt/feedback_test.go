package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"podmutt/internal/control"
	"podmutt/internal/feedback"
)

func TestCLIFeedbackRedactsArgumentsAndUnknownErrors(t *testing.T) {
	for _, args := range [][]string{{"--version=private-secret"}, {"--unknown=private-secret"}, nil} {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, func() error { return errors.New("https://user:private-secret@example.test/?token=private-secret") }, nil)
		if code == 0 || strings.Contains(stderr.String(), "private-secret") {
			t.Fatal(code, stderr.String())
		}
	}
}

func TestNewerDatabaseGuidanceAndCLINotRunningExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr, func() error {
		return feedback.New(feedback.NewerDatabase, "Database version 99 is newer than supported version 3.")
	}, nil)
	for _, want := range []string{"Upgrade podmutt", "newer binary", "version 99", "version 3", "Do not delete"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatal(stderr.String())
		}
	}
	if code != 1 {
		t.Fatal(code)
	}
	stderr.Reset()
	code = run([]string{"play"}, &stdout, &stderr, nil, func(string, io.Writer) error { return control.ErrNotRunning })
	if code != 3 || !strings.Contains(stderr.String(), "Start podmutt in a terminal") {
		t.Fatal(code, stderr.String())
	}
}
