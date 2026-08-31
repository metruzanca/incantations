// Package cpu reports CPU utilization, load average, and the processes
// consuming the most CPU. Utilization is measured by sampling /proc counter
// deltas over a short window.
package cpu

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
	"github.com/metruzanca/incantations/internal/units"
)

// userHZ is Linux's USER_HZ clock tick rate. A core running at 100% consumes
// userHZ ticks per second of wall time.
const userHZ = 100

// Limit is the number of top processes shown.
const Limit = 10

// cpuMinPercent is the smallest per-process CPU share worth reporting, which
// keeps sampler noise and idle threads out of the list.
const cpuMinPercent = 0.5

// CPUStat is an aggregate snapshot of /proc/stat's cpu line, in ticks.
type CPUStat struct {
	User, Nice, System  uint64
	Idle, Iowait        uint64
	Irq, Softirq, Steal uint64
}

// Total returns the sum of all measured ticks.
func (c CPUStat) Total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.Iowait + c.Irq + c.Softirq + c.Steal
}

// UsageDeltas converts two samples into user, system, and idle percentages.
func UsageDeltas(before, after CPUStat) (user, system, idle float64) {
	total := int64(after.Total()) - int64(before.Total())
	if total <= 0 {
		return 0, 0, 0
	}
	user = 100 * float64(int64(after.User+after.Nice)-int64(before.User+before.Nice)) / float64(total)
	system = 100 * float64(int64(after.System+after.Irq+after.Softirq+after.Steal)-int64(before.System+before.Irq+before.Softirq+before.Steal)) / float64(total)
	idle = 100 * float64(int64(after.Idle+after.Iowait)-int64(before.Idle+before.Iowait)) / float64(total)
	return user, system, idle
}

// ProcTick is one process's sampled accounting from /proc/<pid>/stat.
type ProcTick struct {
	PID    int
	Name   string
	Utime  uint64
	Stime  uint64
	RSSKiB uint64
}

// Proc is a process with its computed CPU share for display.
type Proc struct {
	PID    int
	Name   string
	CPU    float64 // percent of one core
	RSSKiB uint64
}

// Report bundles the full result of a capture.
type Report struct {
	User, System, Idle float64
	Load               [3]float64 // 1m, 5m, 15m averages
	Window             time.Duration
	Procs              []Proc
}

// ProcDeltas derives per-process CPU usage from two samples.
func ProcDeltas(before, after map[int]ProcTick, elapsed time.Duration) []Proc {
	var procs []Proc
	if elapsed <= 0 {
		return procs
	}
	for pid, prev := range before {
		cur, ok := after[pid]
		if !ok {
			continue
		}
		ticks := int64(cur.Utime+cur.Stime) - int64(prev.Utime+prev.Stime)
		if ticks <= 0 {
			continue
		}
		cpu := 100 * float64(ticks) / (elapsed.Seconds() * userHZ)
		if cpu < cpuMinPercent {
			continue
		}
		procs = append(procs, Proc{PID: pid, Name: cur.Name, CPU: cpu, RSSKiB: cur.RSSKiB})
	}
	sortProcDesc(procs)
	if len(procs) > Limit {
		procs = procs[:Limit]
	}
	return procs
}

func sortProcDesc(procs []Proc) {
	for i := 1; i < len(procs); i++ {
		for j := i; j > 0 && procs[j-1].CPU < procs[j].CPU; j-- {
			procs[j-1], procs[j] = procs[j], procs[j-1]
		}
	}
}

// ParseStat reads the aggregate cpu line (first line) of /proc/stat contents.
func ParseStat(r io.Reader) (CPUStat, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || fields[0] != "cpu" {
			continue
		}
		if len(fields) < 8 {
			return CPUStat{}, fmt.Errorf("short cpu line: %q", sc.Text())
		}
		parse := func(a ...int) uint64 { return atoiOrDefault(fields, a...) }
		return CPUStat{
			User:    parse(1),
			Nice:    parse(2),
			System:  parse(3),
			Idle:    parse(4),
			Iowait:  parse(5),
			Irq:     parse(6),
			Softirq: parse(7),
			Steal:   parse(8),
		}, nil
	}
	if err := sc.Err(); err != nil {
		return CPUStat{}, err
	}
	return CPUStat{}, fmt.Errorf("no aggregate cpu line found")
}

// ParseLoadavg reads the 1m, 5m, and 15m averages from /proc/loadavg contents.
func ParseLoadavg(r io.Reader) ([3]float64, error) {
	var out [3]float64
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		for i := 0; i < 3; i++ {
			v, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				return out, err
			}
			out[i] = v
		}
		return out, nil
	}
	return out, sc.Err()
}

// ParseProcStat reads the accounting fields from /proc/<pid>/stat contents.
// The comm field may contain spaces and parentheses, so the name is taken from
// between the first '(' and the last ')'.
func ParseProcStat(r io.Reader) (ProcTick, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ProcTick{}, err
	}
	s := strings.TrimSpace(string(data))
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close <= open {
		return ProcTick{}, fmt.Errorf("malformed proc stat")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	if err != nil {
		return ProcTick{}, err
	}
	name := s[open+1 : close]
	rest := strings.Fields(s[close+1:])
	if len(rest) < 22 {
		return ProcTick{}, fmt.Errorf("too few fields in proc stat")
	}
	return ProcTick{
		PID:    pid,
		Name:   name,
		Utime:  mustParseUint(rest[11]),
		Stime:  mustParseUint(rest[12]),
		RSSKiB: mustParseUint(rest[21]) * 4, // rss is in pages; 4096-byte pages
	}, nil
}

func atoiOrDefault(fields []string, idx ...int) uint64 {
	if idx[0] < len(fields) {
		if v, err := strconv.ParseUint(fields[idx[0]], 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func mustParseUint(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// Spec registers the cpu command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "cpu",
		Summary: "show CPU utilization and top CPU-consuming processes",
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

// Render formats a report for humans, most CPU-heavy first.
func Render(r *Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CPU usage (last %dms)\n", r.Window.Milliseconds())
	fmt.Fprintf(&b, "%-12s %6.1f%%\n", "Programs", r.User)
	fmt.Fprintf(&b, "%-12s %6.1f%%\n", "System", r.System)
	fmt.Fprintf(&b, "%-12s %6.1f%%\n", "Idle", r.Idle)
	fmt.Fprintf(&b, "%-12s %.2f %.2f %.2f\n", "Load (1m 5m 15m)", r.Load[0], r.Load[1], r.Load[2])
	if len(r.Procs) > 0 {
		b.WriteString("\nTop processes by CPU\n")
		rows := make([][]string, 0, len(r.Procs))
		for _, p := range r.Procs {
			rows = append(rows, []string{
				p.Name,
				strconv.Itoa(p.PID),
				fmt.Sprintf("%.1f%%", p.CPU),
				units.HumanMemory(p.RSSKiB),
			})
		}
		b.WriteString(ui.NewTable(
			[]string{"COMMAND", "PID", "CPU", "MEMORY"},
			[]bool{false, true, true, true},
			rows,
		))
		b.WriteString("\n")
	}
	return b.String()
}
