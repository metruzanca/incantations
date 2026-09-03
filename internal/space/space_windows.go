//go:build windows

package space

import "fmt"

// Sample reports that this utility is not yet implemented on the platform.
func Sample(path string) (*Report, error) {
	return nil, fmt.Errorf("not yet supported on windows")
}
