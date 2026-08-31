// Package sys combines the ram, cpu, and disk reports into one overview.
package sys

import (
	"fmt"
	"io"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/cpu"
	"github.com/metruzanca/incantations/internal/disk"
	"github.com/metruzanca/incantations/internal/ram"
)

// Report bundles the three subsystem reports.
type Report struct {
	RAM  *ram.Report
	CPU  *cpu.Report
	Disk *disk.Report
}

// Spec registers the sys command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "sys",
		Summary: "show RAM, CPU, and disk usage at once",
		Help: `Usage:
  incantations sys

Runs ram, cpu, and disk in one shot and prints the three reports together.
CPU sampling takes a short window (~300ms), so this is a bit slower than any
single command.`,
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

// Sample reads all three system snapshots. Propagates the first error from
// any subsystem so a single unsupported platform fails cleanly.
func Sample() (*Report, error) {
	r, err := ram.Sample()
	if err != nil {
		return nil, fmt.Errorf("ram: %w", err)
	}
	c, err := cpu.Sample()
	if err != nil {
		return nil, fmt.Errorf("cpu: %w", err)
	}
	d, err := disk.Sample()
	if err != nil {
		return nil, fmt.Errorf("disk: %w", err)
	}
	return &Report{RAM: r, CPU: c, Disk: d}, nil
}

// Render concatenates the three sub-reports (with their section headings),
// each ending in a blank line.
func Render(r *Report) string {
	var b strings.Builder
	b.WriteString(strings.TrimRight(ram.Render(r.RAM, true), "\n"))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimRight(cpu.Render(r.CPU, true), "\n"))
	b.WriteString("\n\n")
	b.WriteString(strings.TrimRight(disk.Render(r.Disk, false, true), "\n"))
	b.WriteString("\n")
	return b.String()
}
