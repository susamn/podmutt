package main

import (
	"bytes"
	"errors"
	"io"
	"podmutt/internal/control"
	"testing"
)

func TestControlExitCodes(t *testing.T) {
	for _, test := range []struct {
		args []string
		err  error
		code int
	}{
		{[]string{"status"}, nil, 0}, {[]string{"play"}, control.ErrNotRunning, 3}, {[]string{"pause"}, errors.New("failed"), 1},
		{[]string{"status", "extra"}, nil, 2}, {[]string{"--version", "status"}, nil, 2},
	} {
		var out, errout bytes.Buffer
		called := false
		code := run(test.args, &out, &errout, func() error { t.Fatal("client started UI"); return nil }, func(command string, w io.Writer) error {
			called = true
			io.WriteString(w, "status-output")
			return test.err
		})
		if code != test.code || called != (test.code != 2) {
			t.Fatalf("%v: code=%d called=%v", test.args, code, called)
		}
	}
}
