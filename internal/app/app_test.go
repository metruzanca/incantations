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
	return []command.Entry{mk("cpu"), mk("disk"), mk("ram"), mk("sys")}
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
	for _, want := range []string{"Usage:", "ram", "cpu", "disk", "init", "sys"} {
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

func TestInitNoShellShowsSetup(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/fish")
	out, errOut, code := run(t, "init")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "No shell specified.") {
		t.Errorf("expected a friendly no-shell message, got %q", out)
	}
	for _, want := range []string{"fish", "config.fish", ">>"} {
		if !strings.Contains(out, want) {
			t.Errorf("init setup missing %q:\n%s", want, out)
		}
	}
	if errOut != "" {
		t.Errorf("stderr should be empty, got %q", errOut)
	}
}

func TestInitHelpDetectsShell(t *testing.T) {
	for _, tc := range []struct{ shell, want string }{
		{"/bin/zsh", "~/.zshrc"},
		{"/usr/bin/fish", "~/.config/fish/config.fish"},
		{"/bin/bash", "~/.bashrc"},
	} {
		t.Run(tc.shell, func(t *testing.T) {
			t.Setenv("SHELL", tc.shell)
			out, _, code := run(t, "init", "--help")
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			if !strings.Contains(out, "Your shell looks like") || !strings.Contains(out, tc.want) {
				t.Errorf("init --help should name the shell and its config %q:\n%s", tc.want, out)
			}
		})
	}
}

func TestInitHelpListsAllWhenUndetected(t *testing.T) {
	t.Setenv("SHELL", "")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("ZSH_VERSION", "")
	out, _, code := run(t, "init", "--help")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"bash", "zsh", "fish", ">>"} {
		if !strings.Contains(out, want) {
			t.Errorf("undetected-shell help missing %q", want)
		}
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
	for _, name := range []string{"ram", "cpu", "disk", "sys"} {
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

func TestInitCommandList(t *testing.T) {
	out, _, code := run(t, "init", "bash", "ram", "cpu")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"ram() {", "cpu() {"} {
		if !strings.Contains(out, want) {
			t.Errorf("init command list missing %q\n%s", want, out)
		}
	}
	for _, not := range []string{"disk() {", "space() {", "battery() {", "init() {"} {
		if strings.Contains(out, not) {
			t.Errorf("init command list should not include %q\n%s", not, out)
		}
	}
}

func TestInitCommandListDetectsShell(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")
	out, _, code := run(t, "init", "space")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "space() {") {
		t.Errorf("init with detected shell should emit a space function:\n%s", out)
	}
	if strings.Contains(out, "ram() {") {
		t.Errorf("only requested commands should be wrapped:\n%s", out)
	}
}

func TestInitUnknownCommand(t *testing.T) {
	_, errOut, code := run(t, "init", "bash", "nope")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("stderr should report the unknown command, got %q", errOut)
	}
}

func TestInitRejectsMetaCommand(t *testing.T) {
	_, errOut, code := run(t, "init", "bash", "init")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "meta command") {
		t.Errorf("stderr should reject meta commands, got %q", errOut)
	}
}

func TestInitNoShellDetectedWithCommands(t *testing.T) {
	t.Setenv("SHELL", "")
	t.Setenv("BASH_VERSION", "")
	t.Setenv("ZSH_VERSION", "")
	_, errOut, code := run(t, "init", "ram")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "shell") {
		t.Errorf("stderr should ask for a shell, got %q", errOut)
	}
}

func TestInitSkipsBatteryWhenAbsent(t *testing.T) {
	old := batteryPresent
	batteryPresent = func() bool { return false }
	defer func() { batteryPresent = old }()

	out, _, code := run(t, "init", "bash")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.Contains(out, "battery() {") {
		t.Errorf("battery wrapper should be omitted when no battery is present:\n%s", out)
	}

	batteryPresent = func() bool { return true }
	out, _, _ = run(t, "init", "bash")
	if !strings.Contains(out, "battery() {") {
		t.Errorf("battery wrapper should appear when a battery is present:\n%s", out)
	}
}
