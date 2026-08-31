package ram

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden testdata files")

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

func fixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestParseMeminfo(t *testing.T) {
	m, err := parseMeminfo(fixture(t, "meminfo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := MemInfo{
		TotalKiB:     32746652,
		AvailableKiB: 14673136,
		BuffersKiB:   587852,
		CachedKiB:    12874080,
		UsedKiB:      32746652 - 14673136,
	}
	if m != want {
		t.Errorf("parseMeminfo = %+v, want %+v", m, want)
	}
}

func TestParseMeminfoMissingTotal(t *testing.T) {
	_, err := parseMeminfo(strings.NewReader("SwapTotal: 0 kB\n"))
	if err == nil {
		t.Fatal("expected error when MemTotal is absent")
	}
}

func TestParseMeminfoMalformedLines(t *testing.T) {
	src := "MemTotal: 1000 kB\nBuffers: not-a-number\nMemAvailable: 400 kB\njunk\nCached: 100 kB\n"
	m, err := parseMeminfo(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.TotalKiB != 1000 || m.AvailableKiB != 400 || m.BuffersKiB != 0 {
		t.Errorf("unexpected parse: %+v", m)
	}
	if m.UsedKiB != 600 {
		t.Errorf("UsedKiB = %d, want 600", m.UsedKiB)
	}
}

func TestParseVmRSS(t *testing.T) {
	rss, err := parseVmRSS(fixture(t, "status.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if rss != 1425408 {
		t.Errorf("VmRSS = %d, want 1425408", rss)
	}
}

func TestParseVmRSSMissing(t *testing.T) {
	_, err := parseVmRSS(strings.NewReader("VmPeak: 1 kB\nVmSize: 2 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRender(t *testing.T) {
	rep := &Report{
		Mem: MemInfo{
			TotalKiB:     32746652,
			AvailableKiB: 14673136,
			BuffersKiB:   587852,
			CachedKiB:    12874080,
			UsedKiB:      32746652 - 14673136,
		},
		Procs: []Process{
			{Name: "chrome", RSSKiB: 1425408, Count: 1},
			{Name: "kthreadd", RSSKiB: 2048, Count: 1},
		},
	}
	golden(t, "ram_render.golden", Render(rep))
}

func TestRenderDeterministic(t *testing.T) {
	rep := &Report{Mem: MemInfo{TotalKiB: 1024, AvailableKiB: 512, UsedKiB: 512}}
	if a, b := Render(rep), Render(rep); a != b {
		t.Error("Render must be deterministic")
	}
}

func TestProcsSortedDescending(t *testing.T) {
	procs := []Process{
		{Name: "low", RSSKiB: 100},
		{Name: "high", RSSKiB: 900},
		{Name: "mid", RSSKiB: 500},
	}
	sortProcsDesc(procs)
	got := []uint64{procs[0].RSSKiB, procs[1].RSSKiB, procs[2].RSSKiB}
	if !reflect.DeepEqual(got, []uint64{900, 500, 100}) {
		t.Errorf("expected descending RSS, got %v", got)
	}
}

func TestAggregate(t *testing.T) {
	procs := []Process{
		{Name: "opencode", RSSKiB: 900},
		{Name: "brave", RSSKiB: 400},
		{Name: "opencode", RSSKiB: 300},
		{Name: "opencode", RSSKiB: 200},
		{Name: "chrome", RSSKiB: 700},
	}
	got := aggregate(procs)
	if len(got) != 3 {
		t.Fatalf("got %d groups, want 3", len(got))
	}
	// opencode totals 1400 KiB across 3, chrome 700, brave 400.
	want := []struct {
		rss   uint64
		count int
	}{{1400, 3}, {700, 1}, {400, 1}}
	for i, w := range want {
		if got[i].RSSKiB != w.rss || got[i].Count != w.count {
			t.Errorf("group %d = %s %dKiB x%d, want %dKiB x%d", i, got[i].Name, got[i].RSSKiB, got[i].Count, w.rss, w.count)
		}
	}
}
