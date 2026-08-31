//go:build linux

package ram

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// meminfoPath and procPath are variables so tests can point at fixtures.
var (
	meminfoPath = "/proc/meminfo"
	procRoot    = "/proc"
)

// parseMeminfo parses /proc/meminfo-style contents. Values are in KiB.
func parseMeminfo(r io.Reader) (MemInfo, error) {
	var m MemInfo
	sc := bufio.NewScanner(r)
	set := func(name string, v uint64) {
		switch name {
		case "MemTotal:":
			m.TotalKiB = v
		case "MemAvailable:":
			m.AvailableKiB = v
		case "Buffers:":
			m.BuffersKiB = v
		case "Cached:":
			m.CachedKiB = v
		}
	}
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		set(fields[0], v)
	}
	if err := sc.Err(); err != nil {
		return m, err
	}
	if m.TotalKiB == 0 {
		return m, fmt.Errorf("no MemTotal found")
	}
	m.UsedKiB = m.TotalKiB - m.AvailableKiB
	return m, nil
}

// parseVmRSS extracts the VmRSS (resident set size, in KiB) from
// /proc/<pid>/status-style contents.
func parseVmRSS(r io.Reader) (uint64, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "VmRSS:" {
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, sc.Err()
}

// readProc reads one process's resident memory. procProbe receives a
// status-like reader and a comm reader; it is a variable so tests can fake it.
func readProc(pid int) (Process, bool) {
	base := fmt.Sprintf("%s/%d", procRoot, pid)
	status, err := os.Open(base + "/status")
	if err != nil {
		return Process{}, false
	}
	rss, err := parseVmRSS(status)
	status.Close()
	if err != nil || rss == 0 {
		return Process{}, false
	}
	name, err := os.ReadFile(base + "/comm")
	if err != nil {
		return Process{}, false
	}
	return Process{PID: pid, Name: strings.TrimSpace(string(name)), RSSKiB: rss}, true
}

// readProcs scans /proc for resident memory of every process.
func readProcs() ([]Process, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	procs := make([]Process, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if p, ok := readProc(pid); ok {
			procs = append(procs, p)
		}
	}
	return procs, nil
}

// Sample reads the current memory state on Linux.
func Sample() (*Report, error) {
	f, err := os.Open(meminfoPath)
	if err != nil {
		return nil, err
	}
	m, err := parseMeminfo(f)
	f.Close()
	if err != nil {
		return nil, err
	}
	procs, err := readProcs()
	if err != nil {
		return nil, err
	}
	sortProcsDesc(procs)
	if len(procs) > Limit {
		procs = procs[:Limit]
	}
	return &Report{Mem: m, Procs: procs}, nil
}

// sortProcsDesc orders processes by resident memory, largest first.
func sortProcsDesc(procs []Process) {
	sort.Slice(procs, func(i, j int) bool { return procs[i].RSSKiB > procs[j].RSSKiB })
}
