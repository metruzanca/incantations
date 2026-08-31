// Package ram reports memory totals and the top memory-consuming processes.
package ram

import (
	"fmt"
	"io"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/units"
)

// MemInfo holds the memory figures that back the report.
type MemInfo struct {
	TotalKiB     uint64
	AvailableKiB uint64
	BuffersKiB   uint64
	CachedKiB    uint64
	UsedKiB      uint64
}

// Process is a single process with its resident memory footprint.
type Process struct {
	PID    int
	Name   string
	RSSKiB uint64
}

// Report bundles the full result of a capture.
type Report struct {
	Mem   MemInfo
	Procs []Process
}

// Limit is the number of top processes shown.
const Limit = 10

// Spec registers the ram command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "ram",
		Summary: "show memory totals and top memory-consuming processes",
		Run: func(args []string, stdout io.Writer) error {
			rep, err := Sample()
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, Render(rep))
			return err
		},
	}
}

// Render formats a report for humans.
func Render(r *Report) string {
	var b strings.Builder
	m := r.Mem
	b.WriteString("Memory\n")
	fmt.Fprintf(&b, "%-13s %-8s\n", "Total", units.HumanKiB(m.TotalKiB))
	fmt.Fprintf(&b, "%-13s %-8s %5.1f%%\n", "Used", units.HumanKiB(m.UsedKiB), units.Pct(m.UsedKiB, m.TotalKiB))
	fmt.Fprintf(&b, "%-13s %-8s\n", "Available", units.HumanKiB(m.AvailableKiB))
	fmt.Fprintf(&b, "%-13s %-8s\n", "Buffers/cache", units.HumanKiB(m.BuffersKiB+m.CachedKiB))
	if len(r.Procs) > 0 {
		b.WriteString("\nTop processes by memory\n")
		fmt.Fprintf(&b, "  %-8s %-9s %7s  %s\n", "PID", "RSS", "%VMEM", "COMMAND")
		for _, p := range r.Procs {
			fmt.Fprintf(&b, "  %-8d %-9s %6.1f%%  %s\n", p.PID, units.HumanKiB(p.RSSKiB), units.Pct(p.RSSKiB, m.TotalKiB), p.Name)
		}
	}
	return b.String()
}
