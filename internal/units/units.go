// Package units formats byte quantities and percentages for human-readable
// output shared across commands.
package units

import "fmt"

// Pct returns part as a percentage of whole (0 when whole is 0).
func Pct(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

// HumanMemory renders a KiB count using decimal units (GB, MB), which is what
// people who are not storage engineers expect to see.
func HumanMemory(kib uint64) string {
	b := float64(kib) * 1024
	switch {
	case b >= 1e12:
		return fmt.Sprintf("%.1f TB", b/1e12)
	case b >= 1e9:
		return fmt.Sprintf("%.1f GB", b/1e9)
	case b >= 1e6:
		return fmt.Sprintf("%.1f MB", b/1e6)
	default:
		return fmt.Sprintf("%.0f KB", b/1e3)
	}
}
