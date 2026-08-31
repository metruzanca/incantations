//go:build !windows

package disk

import (
	"bytes"
	"os/exec"
)

// Sample runs `df -hT` (human-readable, with filesystem type) and parses it.
func Sample() (*Report, error) {
	out, err := exec.Command("df", "-hT").Output()
	if err != nil {
		return nil, err
	}
	rows, err := parseDf(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	return &Report{Rows: rows}, nil
}