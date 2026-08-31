package app

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/metruzanca/incantations/internal/command"
)

// sampleUtilities are synthetic non-meta commands used to verify wrapper
// generation without depending on the real utility packages.
func sampleUtilities() []command.Entry {
	mk := func(name string) command.Entry {
		return command.Entry{Name: name, Summary: "synthetic utility", Run: func(args []string, stdout io.Writer) error { return nil }}
	}
	return []command.Entry{mk("cpu"), mk("disk"), mk("ram")}
}

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	code = New(&out, &errb, sampleUtilities()...).Run(args)
	return out.String(), errb.String(), code
}

func TestNoArgsPrintsUsage(t *testing.T) {
	out, _, code := run(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"Usage:", "ram", "cpu", "disk", "init"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage missing %q", want)
		}
	}
}

func TestHelp(t *testing.T) {
	out, _, code := run(t, "help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "Commands:") {
		t.Errorf("help output missing command list")
	}
	if strings.Contains(out, "unknown command") {
		t.Errorf("help must not print an error")
	}
}

func TestVersion(t *testing.T) {
	out, _, code := run(t, "--version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.HasPrefix(out, "incantations ") {
		t.Errorf("version output = %q", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	out, errOut, code := run(t, "nope")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("stderr missing error, got %q", errOut)
	}
	if out != "" {
		t.Errorf("stdout must be empty on error, got %q", out)
	}
}

func TestInitWrapsUtilities(t *testing.T) {
	out, _, code := run(t, "init", "bash")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"ram() {", "disk() {", "cpu() {", "incantations ram \"$@\""} {
		if !strings.Contains(out, want) {
			t.Errorf("init bash missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "init() {") {
		t.Errorf("meta commands must not be wrapped:\n%s", out)
	}
	if !strings.HasPrefix(out, "#") {
		t.Errorf("init output must start with a comment")
	}
}

func TestInitShellTakesPrecedence(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		if out, _, code := run(t, "init", shell); code != 0 {
			t.Fatalf("init %s exit = %d", shell, code)
		} else if out == "" {
			t.Errorf("init %s produced no output", shell)
		}
	}
}

func TestInitMissingShell(t *testing.T) {
	_, errOut, code := run(t, "init")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "usage: incantations init") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestInitUnsupportedShell(t *testing.T) {
	_, errOut, code := run(t, "init", "tcsh")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "bash, zsh, fish") {
		t.Errorf("stderr should name supported shells, got %q", errOut)
	}
	out, _, _ := run(t, "init", "tcsh")
	if out != "" {
		t.Errorf("stdout must stay clean on error, got %q", out)
	}
}

func TestInitDeterministic(t *testing.T) {
	a, _, _ := run(t, "init", "bash")
	b, _, _ := run(t, "init", "bash")
	if a != b {
		t.Error("init output must be deterministic")
	}
}

func TestCommandHelp(t *testing.T) {
	for _, name := range []string{"ram", "cpu", "disk"} {
		for _, flag := range []string{"--help", "-h"} {
			out, errOut, code := run(t, name, flag)
			if code != 0 {
				t.Fatalf("%s %s exit = %d, want 0", name, flag, code)
			}
			for _, want := range []string{"incantations " + name + " -", "Usage:"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s %s missing %q:\n%s", name, flag, want, out)
				}
			}
			if strings.Contains(out, "unknown command") {
				t.Errorf("%s %s must not print an error", name, flag)
			}
			if errOut != "" {
				t.Errorf("%s %s stderr should be empty, got %q", name, flag, errOut)
			}
		}
	}
}

func TestCommandHelpDoesNotRun(t *testing.T) {
	ran := false
	entry := command.Entry{
		Name: "probe", Summary: "test",
		Run: func(args []string, stdout io.Writer) error { ran = true; return nil },
	}
	var out, errb bytes.Buffer
	code := New(&out, &errb, entry).Run([]string{"probe", "--help"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if ran {
		t.Error("--help must not execute the command")
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("fallback help should include Usage, got %q", out.String())
	}
}

func TestInitHelp(t *testing.T) {
	out, _, code := run(t, "init", "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"incantations init -", "Usage:", "bash", "zsh", "fish", "eval"} {
		if !strings.Contains(out, want) {
			t.Errorf("init --help missing %q", want)
		}
	}
}

func TestRunCommandPassesArgsThrough(t *testing.T) {
	var got []string
	entry := command.Entry{
		Name:    "echoer",
		Summary: "test",
		Run: func(args []string, stdout io.Writer) error {
			got = args
			_, err := io.WriteString(stdout, "ok")
			return err
		},
	}
	var out, errb bytes.Buffer
	code := New(&out, &errb, entry).Run([]string{"echoer", "-x", "y"})
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(got) != 2 || got[0] != "-x" || got[1] != "y" {
		t.Errorf("args = %v, want [-x y]", got)
	}
	if out.String() != "ok" {
		t.Errorf("stdout = %q, want ok", out.String())
	}
}

func TestRunCommandError(t *testing.T) {
	entry := command.Entry{
		Name: "boom", Summary: "test",
		Run: func(args []string, stdout io.Writer) error { return io.ErrUnexpectedEOF },
	}
	var out, errb bytes.Buffer
	code := New(&out, &errb, entry).Run([]string{"boom"})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	outStr, errStr := out.String(), errb.String()
	if outStr != "" {
		t.Errorf("stdout should be empty on error, got %q", outStr)
	}
	if !strings.Contains(errStr, "boom") || !strings.Contains(errStr, "unexpected EOF") {
		t.Errorf("stderr = %q", errStr)
	}
}
