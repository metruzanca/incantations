package space

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestParseDu(t *testing.T) {
	rep, err := parseDu(fixture(t, "du.txt"), "/mnt/data")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Path != "/mnt/data" || rep.Total != 229371207680 {
		t.Errorf("root = %s total=%d", rep.Path, rep.Total)
	}
	if len(rep.Rows) != 4 {
		t.Fatalf("parsed %d dirs, want 4", len(rep.Rows))
	}
	// The root's own line must be consumed as the total, not a row.
	for _, r := range rep.Rows {
		if r.Path == "/mnt/data" {
			t.Errorf("root line leaked into rows: %+v", r)
		}
	}
	if rep.Rows[0].Path != "/mnt/data/media" || rep.Rows[0].Bytes != 131941395333 {
		t.Errorf("first dir = %+v", rep.Rows[0])
	}
}

func TestRender(t *testing.T) {
	rep, err := parseDu(fixture(t, "du.txt"), "/mnt/data")
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "space_render.golden", Render(rep, false))
}

func TestRenderDeterministic(t *testing.T) {
	rep, err := parseDu(fixture(t, "du.txt"), "/mnt/data")
	if err != nil {
		t.Fatal(err)
	}
	if a, b := Render(rep, false), Render(rep, false); a != b {
		t.Error("Render must be deterministic")
	}
}

func TestRenderHidesSmallUnlessAll(t *testing.T) {
	rep := &Report{
		Path:  "/",
		Total: 1 << 40,
		Rows:  []Dir{{Path: "/big", Bytes: 900 << 30}, {Path: "/small", Bytes: 500 << 20}},
	}
	def := Render(rep, false)
	if strings.Contains(def, "small") {
		t.Error("small directory should be hidden by default")
	}
	all := Render(rep, true)
	if !strings.Contains(all, "small") {
		t.Error("-a should show small directories")
	}
}
