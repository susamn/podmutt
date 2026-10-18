//go:build windows

package theme

import "os"

// NotifyReload is a no-op on non-POSIX platforms.
func NotifyReload(ch chan<- os.Signal) {}
