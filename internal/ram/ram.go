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
	SwapTotalKiB uint64
	SwapUsedKiB  uint64
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

Shows RAM and swap usage with a usage bar, plus the processes using the most
memory. Processes are grouped by name.`,
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

// Render formats a report for humans. Section headings ("Memory",
// "Top processes by memory") are only emitted when sectioned, so sys can
// label its combined output while individual invocations stay clean. RAM and
// swap (when present) share one table, each row a progress bar plus a summary
// of the form "used/total (free)".
func Render(r *Report, sectioned bool) string {
	var b strings.Builder
	m := r.Mem
	if sectioned {
		b.WriteString("Memory\n")
	}
	rows := [][]string{{
		"RAM",
		usageCell(units.Pct(m.UsedKiB, m.TotalKiB), m.UsedKiB, m.TotalKiB, m.AvailableKiB),
	}}
	if m.SwapTotalKiB > 0 {
		rows = append(rows, []string{
			"SWAP",
			usageCell(units.Pct(m.SwapUsedKiB, m.SwapTotalKiB), m.SwapUsedKiB, m.SwapTotalKiB, m.SwapTotalKiB-m.SwapUsedKiB),
		})
	}
	b.WriteString(ui.NewTable(
		[]string{"TYPE", "USAGE"},
		[]bool{false, false},
		rows,
	))
	b.WriteString("\n")
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

// usageCell renders a progress bar, the usage percentage, and the
// "used/total (free)" summary for one memory block.
func usageCell(pct float64, used, total, free uint64) string {
	return fmt.Sprintf("%s %3.0f%%  %s/%s (%s Free)",
		ui.Bar(pct/100, 20), pct,
		units.CompactKiB(used), units.CompactKiB(total), units.CompactKiB(free))
}
