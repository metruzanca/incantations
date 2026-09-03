//go:build !linux

package net

import (
	"fmt"
	"runtime"
	"time"
)

// Sample reports that network measurement is not supported on this platform.
func Sample(interval time.Duration) (*Report, error) {
	return nil, fmt.Errorf("not yet supported on %s", runtime.GOOS)
}
