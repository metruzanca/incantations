// Package disk reports filesystem usage by parsing `df -hT` output.
package disk

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
)

// Row is a single parsed df entry for one filesystem.
type Row struct {
	Filesystem string
	Type       string
	Size       string
	Used       string
	Avail      string
	UsePct     float64
	Mount      string
}

// Report is the full set of parsed filesystems.
type Report struct {
	Rows []Row
}

// hiddenTypes lists virtual filesystems that carry no real disk and only
// clutter output. Opinionated: users care about real disks, network mounts,
// and things like /boot.
var hiddenTypes = map[string]bool{
	"autofs": true, "bpf": true, "cgroup": true, "cgroup2": true,
	"configfs": true, "debugfs": true, "devpts": true, "devtmpfs": true,
	"efivarfs": true, "fusectl": true, "hugetlbfs": true, "mqueue": true,
	"nsfs": true, "overlay": true, "proc": true, "pstore": true,
	"ramfs": true, "rpc_pipefs": true, "securityfs": true, "squashfs": true,
	"sysfs": true, "tmpfs": true, "tracefs": true,
}

// minVisibleSize is the smallest filesystem shown by default, matching df -h
// units (1 GiB). Small partitions such as /boot stay hidden unless -a is used.
const minVisibleSize = 1 << 30

// sizeValue parses a df -h size string into its numeric value on the same
// scale (K/M/G/T are powers of 1024, matching GNU df's -h output).
func sizeValue(s string) float64 {
	if s == "" {
		return 0
	}
	mult := 1.0
	num := s
	if last := s[len(s)-1]; strings.ContainsAny(string(last), "KMGTP") {
		num = s[:len(s)-1]
		switch last {
		case 'K':
			mult = 1 << 10
		case 'M':
			mult = 1 << 20
		case 'G':
			mult = 1 << 30
		case 'T':
			mult = 1 << 40
		case 'P':
			mult = 1 << 50
		}
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return v * mult
}

// Sample executes df and returns the report. Platform-specific; defined in
// platform files. Not declared here because the implementation lives solely in
// build-tagged files.

// Spec registers the disk command. -a and --all show small filesystems that
// would otherwise be hidden.
func Spec() command.Entry {
	return command.Entry{
		Name:    "disk",
		Summary: "show disk usage for real filesystems (use -a to show all)",
		Run: func(args []string, stdout io.Writer) error {
			showAll := false
			for _, a := range args {
				switch a {
				case "-a", "--all":
					showAll = true
				default:
					return fmt.Errorf("usage: incantations disk [-a|--all]")
				}
			}
			rep, err := Sample()
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, Render(rep, showAll))
			return err
		},
	}
}

// parseDf parses `df -hT`-style output into rows.
func parseDf(r io.Reader) ([]Row, error) {
	var rows []Row
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "Filesystem") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		use, err := strconv.ParseFloat(strings.TrimSuffix(fields[5], "%"), 64)
		if err != nil {
			continue
		}
		rows = append(rows, Row{
			Filesystem: fields[0],
			Type:       fields[1],
			Size:       fields[2],
			Used:       fields[3],
			Avail:      fields[4],
			UsePct:     use,
			Mount:      strings.Join(fields[6:], " "),
		})
	}
	return rows, sc.Err()
}

// Render formats the report for humans, most full first. Small filesystems
// are hidden unless showAll is set.
func Render(r *Report, showAll bool) string {
	rows := append([]Row(nil), r.Rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].UsePct > rows[j].UsePct })
	var out [][]string
	for _, row := range rows {
		if hiddenTypes[row.Type] {
			continue
		}
		if !showAll && sizeValue(row.Size) < minVisibleSize {
			continue
		}
		out = append(out, []string{
			row.Filesystem,
			row.Type,
			row.Size,
			row.Used,
			row.Avail,
			usageCell(row.UsePct),
			row.Mount,
		})
	}
	var b strings.Builder
	b.WriteString("Disk usage\n")
	b.WriteString(ui.NewTable(
		[]string{"FILESYSTEM", "TYPE", "SIZE", "USED", "AVAILABLE", "USAGE", "MOUNTED ON"},
		[]bool{false, false, true, true, true, true, false},
		out,
	))
	b.WriteString("\n")
	return b.String()
}

// usageCell renders a progress bar plus the numeric usage for one filesystem.
func usageCell(pct float64) string {
	return ui.ProgressBar(pct/100, 12) + " " + fmt.Sprintf("%.0f%%", pct)
}
