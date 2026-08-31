package shell

import (
	"flag"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden testdata files")

// The command list is fixed here so golden files are independent of the
// registry state; app tests cover end-to-end registry generation.
var testCommands = []string{"cpu", "disk", "ram"}

func TestGenerateUnsupported(t *testing.T) {
	_, err := Generate("tcsh", testCommands)
	if err == nil {
		t.Fatal("expected error for tcsh")
	}
	if !strings.Contains(err.Error(), "tcsh") {
		t.Fatalf("error should mention the shell, got %q", err)
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"bash":     "bash",
		"BASH":     "bash",
		"/bin/zsh": "zsh",
		" zsh ":    "zsh",
		"fish":     "fish",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGoldenExact(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			got, err := Generate(shell, testCommands)
			if err != nil {
				t.Fatalf("Generate(%q): %v", shell, err)
			}
			golden(t, shell+".golden", got)
		})
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := "testdata/" + name
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

func TestDeterminismAndIdempotency(t *testing.T) {
	first, err := Generate("bash", testCommands)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate("bash", testCommands)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("generation must be deterministic across calls")
	}
	if !strings.Contains(first, "incantations ram") || !strings.Contains(first, "incantations cpu") {
		t.Fatal("expected ram and cpu wrappers in bash output")
	}
}
