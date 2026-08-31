// Package ram reports memory totals and the top memory-consuming processes.
package ram

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
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

// Process is a group of processes sharing a command name, with their combined
// resident memory footprint.
type Process struct {
	Name   string
	RSSKiB uint64
	Count  int
}

// Report bundles the full result of a capture.
type Report struct {
	Mem   MemInfo
	Procs []Process
}

// Limit is the number of top process groups shown.
const Limit = 10

// Spec registers the ram command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "ram",
		Summary: "show memory totals and top memory-consuming processes",
		Help: `Usage:
  incantations ram

Shows total, used, available, and cache memory with a usage bar, plus the
processes using the most memory. Processes are grouped by name.`,
		Run: func(args []string, stdout io.Writer) error {
			rep, err := Sample()
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, Render(rep, false))
			return err
		},
	}
}

// Render formats a report for humans. Section headings ("RAM",
// "Top processes by memory") are only emitted when sectioned, so sys can
// label its combined output while individual invocations stay clean.
func Render(r *Report, sectioned bool) string {
	var b strings.Builder
	m := r.Mem
	usedPct := units.Pct(m.UsedKiB, m.TotalKiB)
	if sectioned {
		b.WriteString("RAM\n")
	}
	fmt.Fprintf(&b, "%s %4.0f%% used\n", ui.Bar(usedPct/100, 20), usedPct)
	fmt.Fprintf(&b, "%-13s %s\n", "Total", units.HumanMemory(m.TotalKiB))
	fmt.Fprintf(&b, "%-13s %s\n", "Used", units.HumanMemory(m.UsedKiB))
	fmt.Fprintf(&b, "%-13s %s\n", "Available", units.HumanMemory(m.AvailableKiB))
	fmt.Fprintf(&b, "%-13s %s\n", "Cache", units.HumanMemory(m.BuffersKiB+m.CachedKiB))
	if len(r.Procs) > 0 {
		b.WriteString("\n")
		if sectioned {
			b.WriteString("Top processes by memory\n")
		}
		rows := make([][]string, 0, len(r.Procs))
		for _, p := range r.Procs {
			rows = append(rows, []string{
				strings.TrimSpace(p.Name),
				units.HumanMemory(p.RSSKiB),
				strconv.Itoa(p.Count),
				fmt.Sprintf("%.1f%%", units.Pct(p.RSSKiB, m.TotalKiB)),
			})
		}
		b.WriteString(ui.NewTable(
			[]string{"COMMAND", "MEMORY", "PROCESSES", "% OF MEMORY"},
			[]bool{false, true, true, true},
			rows,
		))
		b.WriteString("\n")
	}
	return b.String()
}
