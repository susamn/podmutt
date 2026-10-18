//go:build !windows

package theme

import (
	"os"
	"os/signal"
	"syscall"
)

// NotifyReload registers ch to receive SIGUSR1 for live theme reloading.
func NotifyReload(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGUSR1)
}
