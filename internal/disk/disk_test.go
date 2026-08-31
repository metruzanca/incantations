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
	// and sorts by fullness.
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "disk_render.golden", Render(&Report{Rows: rows}))
}

func TestRenderDeterministic(t *testing.T) {
	rows, err := parseDf(fixture(t, "df.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if a, b := Render(&Report{Rows: rows}), Render(&Report{Rows: rows}); a != b {
		t.Error("Render must be deterministic")
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
