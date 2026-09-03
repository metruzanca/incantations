//go:build !linux

package battery

import (
	"fmt"
	"runtime"
)

// Sample reports that batteries are not readable on this platform.
func Sample() (*Report, error) {
	return nil, fmt.Errorf("not yet supported on %s", runtime.GOOS)
}
