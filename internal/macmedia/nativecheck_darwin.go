//go:build darwin && cgo && nativecheck

package macmedia

/*
#cgo CFLAGS: -DPODMUTT_NATIVE_CHECK
#include <stdlib.h>
#include "bridge.h"
*/
import "C"
import "unsafe"

// Only nativecheck builds expose these synthetic diagnostics. They exercise
// the registered Objective-C blocks/CGO ABI, not physical keyboard arbitration.
type capturedHandler struct{ pointer unsafe.Pointer }

func (d *nativeDriver) capture(action int) capturedHandler {
	return capturedHandler{C.pm_check_capture(d.pointer, C.int(action))}
}
func (h capturedHandler) fire() bool { return C.pm_check_fire(h.pointer) != 0 }
func (h capturedHandler) release()   { C.pm_check_release(h.pointer) }
func podmuttMediaEventForTest(id uint64, action int) bool {
	return podmuttMediaEvent(C.uintptr_t(id), C.int(action)) != 0
}
func nativeControls() int { return int(C.pm_check_controls()) }
func nativeState() int    { return int(C.pm_check_state()) }
func nativeTitle() string {
	ptr := C.pm_check_title()
	defer C.free(unsafe.Pointer(ptr))
	return C.GoString(ptr)
}
