// Package app wires the command registry into a dispatcher and provides a
// fully testable entry point. The CLI surface intentionally mirrors the plan:
// subcommands dispatched off os.Args with help and version handling.
package app

import (
	"fmt"
	"io"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/cpu"
	"github.com/metruzanca/incantations/internal/disk"
	"github.com/metruzanca/incantations/internal/logutil"
	"github.com/metruzanca/incantations/internal/ram"
	"github.com/metruzanca/incantations/internal/shell"
	"github.com/metruzanca/incantations/internal/sys"
)

// Version may be overridden at build time with
// -ldflags "-X github.com/metruzanca/incantations/internal/app.Version=v0.1.0".
var Version = "dev"

const concept = "Incantations - small system utilities for people who don't remember the incantation."

// App holds the command registry and output streams.
type App struct {
	reg    *command.Registry
	stdout io.Writer
	stderr io.Writer
}

// New builds an App with the given extra commands registered alongside init.
// The init command is always registered so eval "$(incantations init ...)"
// keeps working as new utilities are added.
func New(stdout, stderr io.Writer, extra ...command.Entry) *App {
	reg := command.New()
	reg.Add(ram.Spec())
	reg.Add(cpu.Spec())
	reg.Add(disk.Spec())
	reg.Add(sys.Spec())
	for _, e := range extra {
		reg.Add(e)
	}
	reg.Add(initSpec(reg))
	return &App{reg: reg, stdout: stdout, stderr: stderr}
}

// Run dispatches args and returns a process exit code.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		a.usage(a.stdout)
		return 0
	}
	switch args[0] {
	case "help", "-h", "--help":
		a.usage(a.stdout)
		return 0
	case "version", "-v", "--version":
		fmt.Fprintf(a.stdout, "incantations %s\n", Version)
		return 0
	}
	logutil.Debugf("dispatch: %q", args)
	entry, ok := a.reg.Get(args[0])
	if !ok {
		fmt.Fprintf(a.stderr, "incantations: unknown command %q\n\n", args[0])
		a.usage(a.stderr)
		return 2
	}
	if wantsHelp(args[1:]) {
		a.commandHelp(a.stdout, entry)
		return 0
	}
	logutil.Debugf("execute %s: args=%q", entry.Name, args[1:])
	if err := entry.Run(args[1:], a.stdout); err != nil {
		logutil.Errorf("%s: %v", entry.Name, err)
		fmt.Fprintf(a.stderr, "incantations %s: %v\n", entry.Name, err)
		return 1
	}
	logutil.Debugf("done %s", entry.Name)
	return 0
}

func (a *App) usage(w io.Writer) {
	fmt.Fprintln(w, concept)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  incantations <command> [args]")
	fmt.Fprintln(w, `  eval "$(incantations init <shell>)"   # install shell functions (bash, zsh, fish)`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	for _, e := range a.reg.List() {
		fmt.Fprintf(w, "  %-10s %s\n", e.Name, e.Summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run incantations <command> --help for details on a command.")
}

// wantsHelp reports whether args request help output.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

// commandHelp prints extended help for one command.
func (a *App) commandHelp(w io.Writer, e command.Entry) {
	help := e.Help
	if help == "" {
		help = "Usage:\n  incantations " + e.Name
	}
	fmt.Fprintf(w, "incantations %s - %s\n\n%s\n", e.Name, e.Summary, strings.TrimRight(help, "\n"))
}

func initSpec(reg *command.Registry) command.Entry {
	return command.Entry{
		Name:    "init",
		Summary: "print shell integration code for your shell",
		Meta:    true,
		Help: `Usage:
  incantations init <bash|zsh|fish>
  eval "$(incantations init bash)"

Prints shell functions so plain "ram", "cpu", and "disk" commands work in
your shell. Add a line like this to your shell's configuration file:

  eval "$(incantations init bash)"   # ~/.bashrc or ~/.zshrc
  incantations init fish | source    # ~/.config/fish/config.fish

Re-running prints the same output, so it is safe to eval again, and new
utilities are picked up automatically.`,
		Run: func(args []string, stdout io.Writer) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: incantations init <%s>", strings.Join(shell.Supported(), "|"))
			}
			names := make([]string, 0, len(reg.List()))
			for _, e := range reg.List() {
				if !e.Meta {
					names = append(names, e.Name)
				}
			}
			src, err := shell.Generate(args[0], names)
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, src+"\n")
			return err
		},
	}
}
