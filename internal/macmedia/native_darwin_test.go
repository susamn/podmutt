//go:build darwin && cgo && nativecheck

package macmedia

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	code := 0
	if os.Getenv("PODMUTT_TEST_NATIVE_MACOS") == "1" {
		if err := Run(func() error { code = m.Run(); return nil }); err != nil {
			code = 1
		}
	} else {
		code = m.Run()
	}
	os.Exit(code)
}
func nativeFixture(t *testing.T) (*Service, *fakeBackend, *nativeDriver) {
	t.Helper()
	if os.Getenv("PODMUTT_TEST_NATIVE_MACOS") != "1" {
		t.Skip("run make native-media-test on macOS")
	}
	b, _ := fixtures()
	s, err := Start(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, b, s.driver.(*nativeDriver)
}
func TestNativeRegisteredObjectiveCCommandsAndLateDelivery(t *testing.T) {
	s, b, d := nativeFixture(t)
	for index, command := range []string{"play", "pause", "toggle", "stop", "next", "seek-forward", "seek-backward"} {
		handler := d.capture(index)
		if handler.pointer == nil {
			t.Fatal("no registered block", command)
		}
		accepted := handler.fire()
		handler.release()
		if !accepted {
			t.Fatal("C -> Go callback failed", command)
		}
		select {
		case got := <-b.calls:
			if got != command {
				t.Fatal(got)
			}
		case <-time.After(time.Second):
			t.Fatal("action lost")
		}
	}
	invalid := d.capture(99)
	if invalid.fire() {
		t.Fatal("invalid native command accepted")
	}
	invalid.release()
	// Invalid command IDs and stale callback IDs must never reach the actor.
	for _, action := range []int{-1, 7, 99} {
		if podmuttMediaEventForTest(d.id, action) {
			t.Fatal("accepted invalid command", action)
		}
	}
	if podmuttMediaEventForTest(d.id+1, 0) {
		t.Fatal("accepted unknown callback ID")
	}
	late := d.capture(2)
	defer late.release()
	s.Close()
	if podmuttMediaEventForTest(d.id, 0) {
		t.Fatal("accepted unregistered callback ID")
	}
	if late.fire() {
		t.Fatal("retained late handler accepted after unregister")
	}
	if _, ok := callbacks.Load(d.id); ok {
		t.Fatal("callback registry leak")
	}
	if nativeControls() != 0 || nativeState() != 0 || nativeTitle() != "" {
		t.Fatal("system controls/metadata not cleared")
	}
	select {
	case command := <-b.calls:
		t.Fatal("late action", command)
	default:
	}
}
func TestNativeMetadataCapabilitiesAndRepeatedRegistration(t *testing.T) {
	s, _, d := nativeFixture(t)
	for _, item := range []struct {
		data        metadata
		state, mask int
		title       string
	}{
		{metadata{Title: "Native check", HasCurrent: true, Playing: true, CanNext: true, Position: 42, Duration: 120}, 1, 2 | 4 | 8 | 16, "Native check"},
		{metadata{Title: "Native check", HasCurrent: true, Playing: true, Position: 42, Duration: 120}, 1, 2 | 4 | 8, "Native check"},
		{metadata{Title: "Native check", HasCurrent: true, Playing: true, CanNext: true, Position: 42, Duration: 120}, 1, 2 | 4 | 8 | 16, "Native check"},
		{metadata{Title: "Paused", HasCurrent: true, Paused: true, CanNext: true}, 2, 1 | 4 | 8 | 16, "Paused"},
		{metadata{Title: "Paused", HasCurrent: true, Paused: true, CanSeek: true}, 2, 1 | 4 | 8 | 512 | 1024, "Paused"},
		{metadata{Title: "Paused", HasCurrent: true, Paused: true}, 2, 1 | 4 | 8, "Paused"},
		{metadata{Title: "Stopped", HasCurrent: true}, 0, 1 | 4 | 8, "Stopped"},
		{metadata{}, 0, 0, ""},
	} {
		if err := d.Update(item.data); err != nil {
			t.Fatal(err)
		}
		if nativeControls() != item.mask || nativeState() != item.state || nativeTitle() != item.title {
			t.Fatalf("native state mismatch: controls=%d state=%d title=%q", nativeControls(), nativeState(), nativeTitle())
		}
	}
	called := false
	if err := Run(func() error { called = true; return nil }); !errors.Is(err, ErrUnavailable) || called {
		t.Fatal("nested event loop allowed", err)
	}
	s.Close()
	b, _ := fixtures()
	again, err := Start(context.Background(), b)
	if err != nil {
		t.Fatal("registration after teardown", err)
	}
	again.Close()
}
