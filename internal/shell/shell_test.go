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

func TestDetectShell(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"SHELL": "/usr/bin/fish"}, "fish"},
		{map[string]string{"SHELL": "/bin/zsh"}, "zsh"},
		{map[string]string{"SHELL": "/bin/bash"}, "bash"},
		{map[string]string{"SHELL": "/bin/sh"}, "bash"},
		{map[string]string{"SHELL": "/usr/bin/tcsh"}, ""},
		{map[string]string{"SHELL": ""}, ""},
		{map[string]string{"SHELL": "", "ZSH_VERSION": "5.9"}, "zsh"},
		{map[string]string{"SHELL": "", "BASH_VERSION": "5.2"}, "bash"},
		{map[string]string{}, ""},
	}
	for _, tc := range cases {
		getenv := func(k string) string { return tc.env[k] }
		if got := DetectShell(getenv); got != tc.want {
			t.Errorf("DetectShell(%v) = %q, want %q", tc.env, got, tc.want)
		}
	}
}

func TestConfigPathAndSetupCommand(t *testing.T) {
	cases := []struct {
		shell, config, setup string
	}{
		{"bash", "~/.bashrc", `echo 'eval "$(incantations init bash)"' >> ~/.bashrc`},
		{"zsh", "~/.zshrc", `echo 'eval "$(incantations init zsh)"' >> ~/.zshrc`},
		{"fish", "~/.config/fish/config.fish", "echo 'incantations init fish | source' >> ~/.config/fish/config.fish"},
	}
	for _, tc := range cases {
		if got := ConfigPath(tc.shell); got != tc.config {
			t.Errorf("ConfigPath(%q) = %q, want %q", tc.shell, got, tc.config)
		}
		if got := SetupCommand(tc.shell); got != tc.setup {
			t.Errorf("SetupCommand(%q) = %q, want %q", tc.shell, got, tc.setup)
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
