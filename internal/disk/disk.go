// Package disk reports filesystem usage by parsing `df -hT` output.
package disk

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/metruzanca/incantations/internal/command"
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

// Sample executes df and returns the report. Platform-specific; defined in
// platform files. Not declared here because the implementation lives solely in
// build-tagged files.

// Spec registers the disk command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "disk",
		Summary: "show disk usage for real filesystems",
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

// Render formats the report for humans, most full first.
func Render(r *Report) string {
	rows := append([]Row(nil), r.Rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].UsePct > rows[j].UsePct })
	var b strings.Builder
	b.WriteString("Disk usage\n")
	w := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "Filesystem\tType\tSize\tUsed\tAvail\tUse%\tMounted on")
	for _, row := range rows {
		if hiddenTypes[row.Type] {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%5.0f%%\t%s\n", row.Filesystem, row.Type, row.Size, row.Used, row.Avail, row.UsePct, row.Mount)
	}
	w.Flush()
	return b.String()
}
