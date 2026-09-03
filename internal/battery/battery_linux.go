//go:build linux

package battery

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// powerRoot is a variable so tests can point at fixtures. It mirrors the
// /sys/class/power_supply directory, one subdirectory per charger/battery.
var powerRoot = "/sys/class/power_supply"

// Sample reads the first battery found under /sys/class/power_supply. A system
// with no battery (most desktops) reports Found=false with no error; anything
// that looks like a real hardware failure still returns an error.
func Sample() (*Report, error) {
	entries, err := os.ReadDir(powerRoot)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(powerRoot, e.Name())
		typ, err := os.ReadFile(filepath.Join(dir, "type"))
		if err != nil || strings.TrimSpace(string(typ)) != "Battery" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "uevent"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		b, err := parseUevent(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		return &Report{Found: true, B: b}, nil
	}
	return &Report{}, nil
}
