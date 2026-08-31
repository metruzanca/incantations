//go:build !linux

package cpu

import (
	"fmt"
	"runtime"
)

// Sample reports that this utility is not yet implemented on the platform.
func Sample() (*Report, error) {
	return nil, fmt.Errorf("not yet supported on %s", runtime.GOOS)
}
