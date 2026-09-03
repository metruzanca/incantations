// Package space reports which directories are using the most disk space under
// a path, by parsing `du` output.
package space

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
)

// Dir is one directory measured by du.
type Dir struct {
	Path  string // as printed by du, e.g. /home/metru/downloads
	Bytes uint64
}

// Report bundles a directory scan of one filesystem tree.
type Report struct {
	Path  string // the scanned root
	Total uint64 // total bytes in the tree, including files in Path itself
	Rows  []Dir
}

// minVisibleSize is the smallest directory shown by default, matching the
// disk command's mbthreshold (1 GiB).
const minVisibleSize = 1 << 30

// parseDu parses `du -x -B1 --max-depth=1` output: one "bytes\tpath" line per
// entry. GNU du prints the root argument itself last, so the root total is
// matched by path rather than by position; every other line is an immediate
// subdirectory.
func parseDu(r io.Reader, root string) (*Report, error) {
	rep := &Report{Path: root}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		i := strings.IndexByte(line, '\t')
		if i < 0 {
			continue
		}
		b, err := strconv.ParseUint(line[:i], 10, 64)
		if err != nil {
			continue
		}
		path := line[i+1:]
		if path == root {
			rep.Total = b
			continue
		}
		rep.Rows = append(rep.Rows, Dir{Path: path, Bytes: b})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rep, nil
}

// Spec registers the space command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "space",
		Summary: "show which directories are using the most disk space",
		Help: `Usage:
  incantations space [PATH] [-a|--all]

Shows the biggest directories under PATH, each with a bar showing its share
of the whole tree. Without PATH it scans your home directory, so a bare space
is the "what's in my home" incantation. Replaces du -xhd1 ~ | sort -rh | head.

Walking every file makes this slow on large directories — tens of seconds on
a full home, longer over a network mount. Entries under 1 GB are hidden by
default; pass -a or --all to include them.`,
		Run: func(args []string, stdout io.Writer) error {
			showAll := false
			path := ""
			for _, a := range args {
				switch {
				case a == "-a" || a == "--all":
					showAll = true
				case strings.HasPrefix(a, "-"):
					return fmt.Errorf("usage: incantations space [PATH] [-a|--all]")
				default:
					path = a
				}
			}
			if path == "" {
				var err error
				path, err = os.UserHomeDir()
				if err != nil {
					return err
				}
			}
			rep, err := Sample(path)
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, Render(rep, showAll))
			return err
		},
	}
}

// Render formats a report as a table of the biggest directories, each bar
// showing its share of the total. Entries under 1 GB are hidden unless
// showAll is set, matching the disk command. Sorted by size.
func Render(r *Report, showAll bool) string {
	rows := append([]Dir(nil), r.Rows...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Bytes > rows[j].Bytes })
	var out [][]string
	for _, row := range rows {
		if !showAll && row.Bytes < minVisibleSize {
			continue
		}
		if row.Bytes == 0 {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(row.Path, r.Path), "/")
		if name == "" {
			continue
		}
		out = append(out, []string{name, dirCell(row.Bytes, r.Total)})
	}
	padColumn(out, 1)
	var b strings.Builder
	fmt.Fprintf(&b, "%s used in %s\n", humanBytes(r.Total), r.Path)
	b.WriteString(ui.NewTable(
		[]string{"DIRECTORY", "USAGE"},
		[]bool{false, false},
		out,
	))
	b.WriteString("\n")
	return b.String()
}

// dirCell renders a progress bar of a directory's share of the whole tree,
// plus the percentage and the directory's size.
func dirCell(bytes, total uint64) string {
	share := float64(bytes) / float64(total)
	return fmt.Sprintf("%s %3.0f%%  %s",
		ui.Bar(share, 20), share*100, humanBytes(bytes))
}

// padColumn right-pads the given column so every row starts the next column
// at the same position (the table truncates but does not pad cells).
func padColumn(rows [][]string, col int) {
	w := 0
	for _, r := range rows {
		if l := ansi.StringWidth(ansi.Strip(r[col])); l > w {
			w = l
		}
	}
	for i := range rows {
		rows[i][col] += strings.Repeat(" ", w-ansi.StringWidth(ansi.Strip(rows[i][col])))
	}
}

// humanBytes renders a byte count in decimal units (G/T), matching the sizes
// df shows so a directory scan reads the same as the partition view.
func humanBytes(b uint64) string {
	f := float64(b)
	switch {
	case f >= 1e12:
		return fmt.Sprintf("%.1fT", f/1e12)
	case f >= 1e9:
		return fmt.Sprintf("%.1fG", f/1e9)
	case f >= 1e6:
		return fmt.Sprintf("%.1fM", f/1e6)
	default:
		return fmt.Sprintf("%.1fK", f/1e3)
	}
}
