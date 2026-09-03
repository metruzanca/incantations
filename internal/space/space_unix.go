//go:build !windows

package space

import (
	"bytes"
	"errors"
	"os/exec"
)

// Sample walks the given tree and measures its immediate subdirectories.
// du prints a nonzero exit when it cannot read some directory, which is fine;
// the stdout totals are still usable.
func Sample(path string) (*Report, error) {
	out, err := exec.Command("du", "-x", "-B1", "--max-depth=1", path).Output()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return nil, err
		}
	}
	return parseDu(bytes.NewReader(out), path)
}
