package cpu

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "update golden testdata files")

func fixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	switch {
	case err != nil && !*update:
		t.Fatalf("reading golden %s: %v (regenerate with -update)", path, err)
	case *update:
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		t.Logf("updated %s", path)
	case string(want) != got:
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

func TestParseStat(t *testing.T) {
	st, err := ParseStat(fixture(t, "stat.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := CPUStat{
		User: 17202137, Nice: 1982212, System: 3552998,
		Idle: 297512165, Iowait: 141423, Irq: 1293858, Softirq: 463722,
	}
	if st != want {
		t.Errorf("ParseStat = %+v, want %+v", st, want)
	}
	if st.Total() != 17202137+1982212+3552998+297512165+141423+1293858+463722 {
		t.Errorf("Total mismatch")
	}
}

func TestParseStatNoAggregate(t *testing.T) {
	_, err := ParseStat(strings.NewReader("cpu0 1 2 3 4 5 6 7\n"))
	if err == nil {
		t.Fatal("expected error when the aggregate cpu line is missing")
	}
}

func TestUsageDeltas(t *testing.T) {
	before := CPUStat{User: 100, Idle: 900}
	after := CPUStat{User: 200, Idle: 900}
	user, system, idle := UsageDeltas(before, after)
	if user != 100 || system != 0 || idle != 0 {
		t.Errorf("usage = %v/%v/%v, want 100/0/0", user, system, idle)
	}

	// Iowait counts toward idle; only Iowait moved between the samples, so the
	// whole interval is idle.
	before2 := CPUStat{User: 100, Iowait: 100}
	after2 := CPUStat{User: 100, Iowait: 150, Idle: 100}
	user2, _, idle2 := UsageDeltas(before2, after2)
	total := int64(after2.Total()) - int64(before2.Total())
	if total != 150 {
		t.Fatalf("total delta = %d, want 150", total)
	}
	if user2 != 0 || idle2 != 100 {
		t.Errorf("usage2 = %v/%v, want 0/100", user2, idle2)
	}
}

func TestUsageDeltasNoMovement(t *testing.T) {
	if u, s, i := UsageDeltas(CPUStat{}, CPUStat{}); u != 0 || s != 0 || i != 0 {
		t.Errorf("expected zeros, got %v %v %v", u, s, i)
	}
}

func TestParseLoadavg(t *testing.T) {
	load, err := ParseLoadavg(fixture(t, "loadavg.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := [3]float64{1.39, 1.37, 1.80}
	if load != want {
		t.Errorf("ParseLoadavg = %v, want %v", load, want)
	}
}

func TestParseProcStat(t *testing.T) {
	tick, err := ParseProcStat(fixture(t, "procstat.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if tick.PID != 4321 {
		t.Errorf("PID = %d, want 4321", tick.PID)
	}
	if tick.Name != "web server (worker)" {
		t.Errorf("Name = %q", tick.Name)
	}
	if tick.Utime != 955 || tick.Stime != 381 {
		t.Errorf("times = %d/%d, want 955/381", tick.Utime, tick.Stime)
	}
	if tick.RSSKiB != 1425408 {
		t.Errorf("RSSKiB = %d, want 1425408 (356352 pages * 4)", tick.RSSKiB)
	}
}

func TestParseProcStatMalformed(t *testing.T) {
	if _, err := ParseProcStat(strings.NewReader("garbage no parens")); err == nil {
		t.Error("expected error for malformed stat")
	}
	if _, err := ParseProcStat(strings.NewReader("1 (x)")); err == nil {
		t.Error("expected error for too-few fields")
	}
}

func TestProcDeltas(t *testing.T) {
	before := map[int]ProcTick{
		1: {PID: 1, Name: "busy", Utime: 50, Stime: 50, RSSKiB: 1024},
		2: {PID: 2, Name: "quiet", Utime: 1000, Stime: 1000},
		3: {PID: 3, Name: "half", Utime: 10000, Stime: 0},
	}
	after := map[int]ProcTick{
		1: {PID: 1, Name: "busy", Utime: 150, Stime: 50, RSSKiB: 2048},
		2: {PID: 2, Name: "quiet", Utime: 1000, Stime: 1000},
		3: {PID: 3, Name: "half", Utime: 10050, Stime: 0},
	}
	procs := ProcDeltas(before, after, time.Second)
	// busy: 100 ticks => 100%. half: 50 ticks => 50%. quiet: 0 ticks => dropped.
	if len(procs) != 2 {
		t.Fatalf("procs = %d, want 2", len(procs))
	}
	if procs[0].PID != 1 || procs[0].CPU != 100 {
		t.Errorf("top proc = %+v, want PID 1 at 100%%", procs[0])
	}
	if procs[1].PID != 3 || procs[1].CPU != 50 {
		t.Errorf("second proc = %+v, want PID 3 at 50%%", procs[1])
	}
	if procs[0].RSSKiB != 2048 {
		t.Errorf("RSS KiB should come from the later snapshot, got %d", procs[0].RSSKiB)
	}
}

func TestProcDeltasZeroElapsed(t *testing.T) {
	if got := ProcDeltas(map[int]ProcTick{1: {}}, map[int]ProcTick{1: {}}, 0); got != nil {
		t.Errorf("expected nil/empty for zero elapsed, got %v", got)
	}
}

func TestRender(t *testing.T) {
	rep := &Report{
		User:   25.0,
		System: 5.0,
		Idle:   70.0,
		Load:   [3]float64{1.5, 0.75, 0.5},
		Window: 300 * time.Millisecond,
		Procs: []Proc{
			{PID: 4321, Name: "web server (worker)", CPU: 99.9, RSSKiB: 1425408},
			{PID: 555, Name: "kthreadd", CPU: 1.2, RSSKiB: 2048},
		},
	}
	golden(t, "cpu_render.golden", Render(rep))
}

func TestRenderDeterministic(t *testing.T) {
	rep := &Report{User: 1, System: 2, Idle: 97, Window: time.Second}
	if a, b := Render(rep), Render(rep); a != b {
		t.Error("Render must be deterministic")
	}
}
