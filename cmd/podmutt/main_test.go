package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{name: "version", args: []string{"--version"}, stdout: "podmutt dev\n"},
		{name: "help", args: []string{"--help"}, stderr: "Usage: podmutt"},
		{name: "unknown flag", args: []string{"--unknown"}, code: 2, stderr: "flag provided but not defined"},
		{name: "unexpected argument", args: []string{"unknown"}, code: 2, stderr: "unexpected arguments"},
		{name: "version with argument", args: []string{"--version", "extra"}, code: 2, stderr: "unexpected arguments"},
		{name: "import-opml missing file", args: []string{"import-opml"}, code: 2, stderr: "import-opml requires exactly one file path"},
		{name: "import-opml extra args", args: []string{"import-opml", "one", "two"}, code: 2, stderr: "import-opml requires exactly one file path"},
		{name: "version with import-opml", args: []string{"--version", "import-opml", "file.opml"}, code: 2, stderr: "import-opml requires exactly one file path"},
		{name: "start TUI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			started := false
			if got := run(tt.args, &stdout, &stderr, func() error { started = true; return nil }, nil); got != tt.code {
				t.Fatalf("exit code = %d, want %d", got, tt.code)
			}
			if started != (len(tt.args) == 0) {
				t.Errorf("TUI started = %v", started)
			}
			if got := stdout.String(); got != tt.stdout {
				t.Errorf("stdout = %q, want %q", got, tt.stdout)
			}
			if got := stderr.String(); (tt.stderr == "" && got != "") || !strings.Contains(got, tt.stderr) {
				t.Errorf("stderr = %q, want containing %q", got, tt.stderr)
			}
		})
	}
}

func TestRunReportsStartupFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr, func() error { return errors.New("profile already running") }, nil)
	if code != 1 || !strings.Contains(stderr.String(), "profile already running") {
		t.Fatalf("startup error: code=%d, stderr=%q", code, stderr.String())
	}
}

func TestRunImportOPML(t *testing.T) {
	orig := importOPML
	defer func() { importOPML = orig }()

	calledPath := ""
	importOPML = func(path string, w io.Writer) error {
		calledPath = path
		if path == "error.opml" {
			return errors.New("parse error")
		}
		io.WriteString(w, "import success")
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"import-opml", "test.opml"}, &stdout, &stderr, nil, nil)
	if code != 0 || calledPath != "test.opml" || !strings.Contains(stdout.String(), "import success") {
		t.Fatalf("success: code=%d calledPath=%q stdout=%q stderr=%q", code, calledPath, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"import-opml", "error.opml"}, &stdout, &stderr, nil, nil)
	if code != 1 || !strings.Contains(stderr.String(), "parse error") {
		t.Fatalf("failure: code=%d stderr=%q", code, stderr.String())
	}
}
