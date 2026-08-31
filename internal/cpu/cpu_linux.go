//go:build linux

package cpu

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	statPath    = "/proc/stat"
	loadavgPath = "/proc/loadavg"
	procRoot    = "/proc"
	sleepFunc   = time.Sleep
)

// readStat loads one CPU aggregate snapshot.
func readStat() (CPUStat, error) {
	f, err := os.Open(statPath)
	if err != nil {
		return CPUStat{}, err
	}
	defer f.Close()
	return ParseStat(f)
}

// readLoadavg loads the 1m/5m/15m averages.
func readLoadavg() ([3]float64, error) {
	f, err := os.Open(loadavgPath)
	if err != nil {
		return [3]float64{}, err
	}
	defer f.Close()
	return ParseLoadavg(f)
}

// readProcs scans /proc for process accounting snapshots.
func readProcs() (map[int]ProcTick, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	procs := make(map[int]ProcTick, len(entries))
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		f, err := os.Open(filepath.Join(procRoot, e.Name(), "stat"))
		if err != nil {
			continue // process may have exited
		}
		tick, err := ParseProcStat(f)
		f.Close()
		if err != nil {
			continue
		}
		procs[pid] = tick
	}
	return procs, nil
}

// Sample measures CPU utilization over a short window on Linux.
func Sample() (*Report, error) {
	window := 300 * time.Millisecond
	before, err := readStat()
	if err != nil {
		return nil, err
	}
	procsBefore, err := readProcs()
	if err != nil {
		return nil, err
	}
	sleepFunc(window)
	after, err := readStat()
	if err != nil {
		return nil, err
	}
	procsAfter, err := readProcs()
	if err != nil {
		return nil, err
	}
	load, err := readLoadavg()
	if err != nil {
		return nil, err
	}
	user, system, idle := UsageDeltas(before, after)
	return &Report{
		User:   user,
		System: system,
		Idle:   idle,
		Load:   load,
		Window: window,
		Procs:  ProcDeltas(procsBefore, procsAfter, window),
	}, nil
}
