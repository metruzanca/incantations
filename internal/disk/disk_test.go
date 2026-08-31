package disk

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

func TestParseDf(t *testing.T) {
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 {
		t.Fatalf("parsed %d rows, want 12", len(rows))
	}
	root := rows[0]
	if root.Filesystem != "/dev/nvme0n1p2" || root.Type != "ext4" || root.Mount != "/" {
		t.Errorf("root row = %+v", root)
	}
	if root.UsePct != 24.0 {
		t.Errorf("root UsePct = %v, want 24", root.UsePct)
	}
	last := rows[len(rows)-1]
	if last.Filesystem != "/dev/sdb" || last.Mount != "/mnt/media with spaces" {
		t.Errorf("mount with spaces not preserved: %+v", last)
	}
}

func TestParseDfSkipsHeaderAndBlank(t *testing.T) {
	src := "Filesystem Type Size Used Avail Use% Mounted on\n\n/dev/foo ext4 10G 1G 9G 10% /\n\n"
	rows, err := parseDf(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("parsed %d rows, want 1", len(rows))
	}
}

func TestRender(t *testing.T) {
	// Parser output feeds the renderer; the render hides virtual filesystems
	// and small partitions, and sorts by fullness.
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "disk_render.golden", Render(&Report{Rows: rows}, false))
}

func TestRenderAll(t *testing.T) {
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "disk_render_all.golden", Render(&Report{Rows: rows}, true))
}

func TestRenderDeterministic(t *testing.T) {
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if a, b := Render(&Report{Rows: rows}, false), Render(&Report{Rows: rows}, false); a != b {
		t.Error("Render must be deterministic")
	}
}

func TestSizeValue(t *testing.T) {
	cases := map[string]float64{
		"1022M": 1022 * 1 << 20,
		"1.7T":  1.7 * (1 << 40),
		"4.0T":  4.0 * (1 << 40),
		"500G":  500 * 1 << 30,
		"128K":  128 * 1 << 10,
		"1":     1,
		"":      0,
		"junk":  0,
	}
	for in, want := range cases {
		if got := sizeValue(in); got != want {
			t.Errorf("sizeValue(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSmallFilesystemsHiddenUnlessAll(t *testing.T) {
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defaultOut := Render(&Report{Rows: rows}, false)
	if strings.Contains(defaultOut, "/boot") {
		t.Error("/boot (1022M) should be hidden by default")
	}
	if !strings.Contains(defaultOut, "hidden") && !strings.Contains(defaultOut, "-a") {
		t.Error("expected a hint when small filesystems are hidden")
	}
	allOut := Render(&Report{Rows: rows}, true)
	if !strings.Contains(allOut, "/boot") {
		t.Error("-a should show /boot")
	}
	if strings.Contains(allOut, "hidden") {
		t.Error("-a output should not claim small filesystems are hidden")
	}
}

func TestHiddenTypes(t *testing.T) {
	if !hiddenTypes["tmpfs"] || !hiddenTypes["overlay"] || !hiddenTypes["squashfs"] {
		t.Error("expected virtual filesystems hidden")
	}
	if hiddenTypes["ext4"] || hiddenTypes["nfs4"] || hiddenTypes["vfat"] || hiddenTypes["btrfs"] {
		t.Error("expected real filesystems visible")
	}
}
