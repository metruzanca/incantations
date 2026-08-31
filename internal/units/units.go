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

// HumanKiB renders a KiB count using binary (1024) units, e.g. 3145728 -> "3.0 GiB".
func HumanKiB(kib uint64) string {
	const unit = 1024
	if kib < unit {
		return fmt.Sprintf("%d KiB", kib)
	}
	exp := 0
	v := float64(kib)
	for v >= unit && exp < 5 {
		v /= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", v, "KMGTPE"[exp])
}
