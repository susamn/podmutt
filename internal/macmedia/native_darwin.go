//go:build darwin && cgo

package macmedia

/*
#cgo CFLAGS: -fblocks -mmacosx-version-min=13.0
#cgo LDFLAGS: -mmacosx-version-min=13.0 -framework AppKit -framework Foundation -framework MediaPlayer
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

var running atomic.Bool

// AppKit must run on the process's original thread. init pins main before it
// can migrate; only the foreground Run callback starts the native event loop.
func init() { runtime.LockOSThread() }
func Run(run func() error) error {
	if !running.CompareAndSwap(false, true) {
		return ErrUnavailable
	}
	defer running.Store(false)
	if C.pm_prepare() == 0 {
		running.Store(false)
		return run()
	}
	done := make(chan error, 1)
	go func() { err := run(); done <- err; C.pm_stop() }()
	C.pm_run()
	return <-done
}

var callbacks sync.Map
var callbackID atomic.Uint64

type nativeDriver struct {
	pointer   unsafe.Pointer
	id        uint64
	installed bool
}

func Start(ctx context.Context, b backend) (*Service, error) {
	if !running.Load() {
		return nil, ErrUnavailable
	}
	return start(ctx, b, &nativeDriver{})
}
func (d *nativeDriver) Install(callback func(string) bool) error {
	d.id = callbackID.Add(1)
	callbacks.Store(d.id, callback)
	d.installed = true
	d.pointer = C.pm_install(C.uintptr_t(d.id))
	if d.pointer == nil {
		return ErrUnavailable
	}
	return nil
}
func (d *nativeDriver) Update(m metadata) error {
	title := C.CString(m.Title)
	defer C.free(unsafe.Pointer(title))
	state := C.int(0)
	if m.Playing {
		state = 1
	} else if m.Paused {
		state = 2
	}
	if C.pm_update(d.pointer, title, C.double(m.Position), C.double(m.Duration), state, flag(m.HasCurrent), flag(m.CanNext)) == 0 {
		return ErrUnavailable
	}
	return nil
}
func flag(value bool) C.int {
	if value {
		return 1
	}
	return 0
}
func (d *nativeDriver) Close() {
	if d.installed {
		callbacks.Delete(d.id)
		C.pm_close(d.pointer)
		d.installed = false
		d.pointer = nil
	}
}

//export podmuttMediaEvent
func podmuttMediaEvent(handle C.uintptr_t, action C.int) C.int {
	commands := []string{"play", "pause", "toggle", "stop", "next"}
	if action < 0 || int(action) >= len(commands) {
		return 0
	}
	value, ok := callbacks.Load(uint64(handle))
	if !ok {
		return 0
	}
	return flag(value.(func(string) bool)(commands[int(action)]))
}
