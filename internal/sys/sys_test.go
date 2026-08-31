package sys

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/metruzanca/incantations/internal/cpu"
	"github.com/metruzanca/incantations/internal/disk"
	"github.com/metruzanca/incantations/internal/ram"
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

func sampleReport() *Report {
	return &Report{
		RAM: &ram.Report{
			Mem: ram.MemInfo{
				TotalKiB:     32746652,
				AvailableKiB: 14673136,
				BuffersKiB:   587852,
				CachedKiB:    12874080,
				UsedKiB:      32746652 - 14673136,
				SwapTotalKiB: 8921084,
				SwapUsedKiB:  8921084 - 3480376,
			},
			Procs: []ram.Process{
				{Name: "chrome", RSSKiB: 1425408, Count: 1},
				{Name: "kthreadd", RSSKiB: 2048, Count: 1},
			},
		},
		CPU: &cpu.Report{
			User:   25.0,
			System: 5.0,
			Idle:   70.0,
			Load:   [3]float64{1.5, 0.75, 0.5},
			Window: 300 * time.Millisecond,
			Procs: []cpu.Proc{
				{PID: 4321, Name: "web server (worker)", CPU: 25.0, RSSKiB: 1425408},
				{PID: 555, Name: "kthreadd", CPU: 1.2, RSSKiB: 2048},
			},
		},
		Disk: &disk.Report{Rows: []disk.Row{
			{Filesystem: "/dev/nvme0n1p2", Type: "ext4", Size: "1.7T", Used: "378G", Avail: "1.3T", UsePct: 24, Mount: "/"},
			{Filesystem: "nfs:/data/shared", Type: "nfs4", Size: "4.0T", Used: "3.2T", Avail: "800G", UsePct: 81, Mount: "/mnt/shared"},
		}},
	}
}

func TestRender(t *testing.T) {
	golden(t, "sys.golden", Render(sampleReport(), false))
	golden(t, "sys_totals.golden", Render(sampleReport(), true))
}

func TestRenderSections(t *testing.T) {
	out := Render(sampleReport(), true)
	for _, section := range []string{"Memory\n", "CPU usage", "Disk usage", "Top processes by CPU", "Top processes by memory"} {
		if !strings.Contains(out, section) {
			t.Errorf("output missing section %q", section)
		}
	}
}

func TestRenderDeterministic(t *testing.T) {
	rep := sampleReport()
	if a, b := Render(rep, false), Render(rep, false); a != b {
		t.Error("Render must be deterministic")
	}
}
