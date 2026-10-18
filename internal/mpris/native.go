package mpris

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

// Start uses an existing Linux session bus: no daemon autolaunch, system-bus
// lookup, or remote transports. Other platforms are an intentional no-op.
func Start(ctx context.Context, backend Backend) (*Service, error) {
	return startOn(ctx, runtime.GOOS, os.Getenv, backend)
}
func startOn(ctx context.Context, platform string, getenv func(string) string, backend Backend) (*Service, error) {
	if platform != "linux" {
		return nil, nil
	}
	address := getenv("DBUS_SESSION_BUS_ADDRESS")
	if address == "" {
		directory := getenv("XDG_RUNTIME_DIR")
		if !filepath.IsAbs(directory) {
			return nil, ErrUnavailable
		}
		address = "unix:path=" + url.PathEscape(filepath.Join(directory, "bus"))
	}
	return Listen(ctx, address, backend)
}
