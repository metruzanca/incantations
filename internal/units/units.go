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
// people who are not storage engineers expect to see. The unit is preceded by
// a space ("33.5 GB").
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

// CompactKiB renders a KiB count like HumanMemory but with no space between
// number and unit, for tight summaries such as "19.2GB/33.5GB".
func CompactKiB(kib uint64) string {
	b := float64(kib) * 1024
	switch {
	case b >= 1e12:
		return fmt.Sprintf("%.1fTB", b/1e12)
	case b >= 1e9:
		return fmt.Sprintf("%.1fGB", b/1e9)
	case b >= 1e6:
		return fmt.Sprintf("%.1fMB", b/1e6)
	default:
		return fmt.Sprintf("%.0fKB", b/1e3)
	}
}

// HumanEnergy renders a microwatt-hour count in watt-hours (or kilowatt-hours
// for big batteries), with a space between number and unit.
func HumanEnergy(uwh uint64) string {
	wh := float64(uwh) / 1e6
	if wh >= 1000 {
		return fmt.Sprintf("%.1f kWh", wh/1000)
	}
	return fmt.Sprintf("%.1f Wh", wh)
}

// CompactEnergy renders a microwatt-hour count with no space between number
// and unit, for tight summaries such as "37.8Wh/79.3Wh".
func CompactEnergy(uwh uint64) string {
	wh := float64(uwh) / 1e6
	if wh >= 1000 {
		return fmt.Sprintf("%.1fkWh", wh/1000)
	}
	return fmt.Sprintf("%.1fWh", wh)
}

// HumanPower renders a microwatt count in watts (or kilowatts), with a space.
func HumanPower(uw uint64) string {
	w := float64(uw) / 1e6
	switch {
	case w >= 1000:
		return fmt.Sprintf("%.1f kW", w/1000)
	case w < 1:
		return fmt.Sprintf("%.0f mW", w*1000)
	default:
		return fmt.Sprintf("%.1f W", w)
	}
}

// HumanDuration renders a number of seconds as a compact "2h 05m", "45m", or
// "30s". Zero seconds renders as "0s".
func HumanDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	if seconds >= 3600 {
		return fmt.Sprintf("%dh %02dm", seconds/3600, seconds%3600/60)
	}
	if seconds >= 60 {
		return fmt.Sprintf("%dm %02ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%ds", seconds)
}

// HumanRate renders a bytes-per-second count using decimal units ("12.5 MB/s"),
// which is what people expect for network speeds.
func HumanRate(bps uint64) string {
	b := float64(bps)
	switch {
	case b >= 1e9:
		return fmt.Sprintf("%.1f GB/s", b/1e9)
	case b >= 1e6:
		return fmt.Sprintf("%.1f MB/s", b/1e6)
	case b >= 1e3:
		return fmt.Sprintf("%.1f KB/s", b/1e3)
	default:
		return fmt.Sprintf("%.0f B/s", b)
	}
}
