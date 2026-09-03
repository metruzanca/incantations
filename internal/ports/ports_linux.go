//go:build linux

package ports

import (
	"bytes"
	"os"
	"os/exec"
	"syscall"
)

// servicesPath is a variable so tests can point at a fixture.
var servicesPath = "/etc/services"

// signalProcess sends a signal to a pid. It is a variable so tests can record
// signals without touching real processes.
var signalProcess = func(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func termSignal() syscall.Signal { return syscall.SIGTERM }
func killSignal() syscall.Signal { return syscall.SIGKILL }

// Sample runs `ss -ltulpn` (listening TCP and UDP with owning processes) and
// reads the service-name table. A missing /etc/services is not fatal.
func Sample() (*Report, error) {
	out, err := exec.Command("ss", "-ltulpn").Output()
	if err != nil {
		return nil, err
	}
	rows, err := parseSs(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	svc := make(map[int]string)
	f, err := os.Open(servicesPath)
	if err == nil {
		parseServices(f, svc)
		f.Close()
	}
	return &Report{Rows: rows, Svc: svc}, nil
}
