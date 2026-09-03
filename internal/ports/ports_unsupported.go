//go:build !linux

package ports

import (
	"fmt"
	"runtime"
	"syscall"
)

// signalProcess always fails: the ports command cannot work off Linux, so
// signaling never gets here, but the stub keeps the package compiling and
// signals from being sent on platforms where syscall.Kill does not exist.
var signalProcess = func(pid int, sig syscall.Signal) error {
	return fmt.Errorf("not yet supported on %s", runtime.GOOS)
}

// termSignal and killSignal resolve to SIGTERM everywhere off Linux; they are
// unreachable because Sample fails first. SIGKILL is not defined on Windows,
// so it is not referenced here.
func termSignal() syscall.Signal { return syscall.SIGTERM }
func killSignal() syscall.Signal { return syscall.SIGTERM }

// Sample reports that port discovery is not supported on this platform.
func Sample() (*Report, error) {
	return nil, fmt.Errorf("not yet supported on %s", runtime.GOOS)
}
