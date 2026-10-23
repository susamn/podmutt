//go:build darwin && cgo

package macmedia

import (
	"context"
	"errors"
	"testing"
)

// Ordinary package tests run without AppKit's event loop. Starting the bridge
// there must not register system handlers or silently report success.
func TestStartOutsideNativeEventLoop(t *testing.T) {
	if running.Load() {
		t.Skip("nativecheck test bundle runs inside the event loop")
	}
	backend, _ := fixtures()
	service, err := Start(context.Background(), backend)
	if service != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("started outside event loop: service=%v err=%v", service, err)
	}
}
