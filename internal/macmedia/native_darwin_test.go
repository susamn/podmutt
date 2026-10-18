//go:build darwin && cgo

package macmedia

import (
	"context"
	"os"
	"testing"
	"time"

	"podmutt/internal/playback"
	"podmutt/internal/store"
)

func TestMain(m *testing.M) {
	code := 0
	if os.Getenv("PODMUTT_TEST_NATIVE_MACOS") == "1" {
		_ = Run(func() error { code = m.Run(); return nil })
	} else {
		code = m.Run()
	}
	os.Exit(code)
}
func TestNativeRegistrationMetadataAndTeardown(t *testing.T) {
	if os.Getenv("PODMUTT_TEST_NATIVE_MACOS") != "1" {
		t.Skip("run make native-media-test on macOS")
	}
	b, _ := fixtures()
	s, err := Start(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := s.driver.(*nativeDriver)
	d.Update(metadata{Title: "Direct native metadata check", Playing: true, HasCurrent: true, Position: 1, Duration: 120})
	b.updates <- playback.State{Status: playback.Playing, Current: &store.Episode{Title: "podmutt native check", PositionSeconds: 1, DurationSeconds: 120}}
	value, ok := callbacks.Load(d.id)
	if !ok {
		t.Fatal("callback not registered")
	}
	if !value.(func(string) bool)("pause") {
		t.Fatal("not accepted")
	}
	select {
	case got := <-b.calls:
		if got != "pause" {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("no shared action")
	}
	s.Close()
	if _, ok := callbacks.Load(d.id); ok {
		t.Fatal("callback leaked")
	}
	// This checks real framework registration/event-loop teardown, not physical
	// media-key delivery or competition with another application.
}
