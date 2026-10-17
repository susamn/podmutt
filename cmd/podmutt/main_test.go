package main

import (
	"bytes"
	"errors"
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
		{name: "unexpected argument", args: []string{"play"}, code: 2, stderr: "unexpected arguments"},
		{name: "version with argument", args: []string{"--version", "extra"}, code: 2, stderr: "unexpected arguments"},
		{name: "start TUI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			started := false
			if got := run(tt.args, &stdout, &stderr, func() error { started = true; return nil }); got != tt.code {
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
	code := run(nil, &stdout, &stderr, func() error { return errors.New("profile already running") })
	if code != 1 || !strings.Contains(stderr.String(), "profile already running") {
		t.Fatalf("startup error: code=%d, stderr=%q", code, stderr.String())
	}
}
