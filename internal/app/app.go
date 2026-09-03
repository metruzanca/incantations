// Package app wires the command registry into a dispatcher and provides a
// fully testable entry point. The CLI surface intentionally mirrors the plan:
// subcommands dispatched off os.Args with help and version handling.
package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/metruzanca/incantations/internal/battery"
	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/cpu"
	"github.com/metruzanca/incantations/internal/disk"
	"github.com/metruzanca/incantations/internal/logutil"
	"github.com/metruzanca/incantations/internal/net"
	"github.com/metruzanca/incantations/internal/ports"
	"github.com/metruzanca/incantations/internal/ram"
	"github.com/metruzanca/incantations/internal/shell"
	"github.com/metruzanca/incantations/internal/space"
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
	reg.Add(battery.Spec())
	reg.Add(ports.Spec())
	reg.Add(net.Spec())
	reg.Add(space.Spec())
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
	if e.HelpFunc != nil {
		help = e.HelpFunc()
	}
	if help == "" {
		help = "Usage:\n  incantations " + e.Name
	}
	fmt.Fprintf(w, "incantations %s - %s\n\n%s\n", e.Name, e.Summary, strings.TrimRight(help, "\n"))
}

// batteryPresent reports whether this machine has a battery, so init can skip
// generating a wrapper for a command that would just say "No battery found."
// It is a variable so tests can pin it either way.
var batteryPresent = battery.HasBattery

func initSpec(reg *command.Registry) command.Entry {
	return command.Entry{
		Name:    "init",
		Summary: "print shell integration code for your shell (optional command list)",
		Meta:    true,
		HelpFunc: func() string {
			return initSetupText()
		},
		Run: func(args []string, stdout io.Writer) error {
			if len(args) == 0 {
				_, err := io.WriteString(stdout, "No shell specified.\n\n"+initSetupText())
				return err
			}
			shellName := ""
			cmdArgs := args
			if shell.IsShell(args[0]) {
				shellName = args[0]
				cmdArgs = args[1:]
			} else {
				shellName = shell.DetectShell(os.Getenv)
				if shellName == "" {
					return fmt.Errorf("couldn't tell which shell you use; pass bash, zsh, or fish followed by the commands to install")
				}
			}
			var names []string
			if len(cmdArgs) > 0 {
				names = make([]string, 0, len(cmdArgs))
				for _, name := range cmdArgs {
					e, ok := reg.Get(name)
					switch {
					case !ok:
						return fmt.Errorf("unknown command %q (shells: bash, zsh, fish)", name)
					case e.Meta:
						return fmt.Errorf("%q is a meta command and cannot be wrapped", name)
					}
					names = append(names, name)
				}
			} else {
				names = wrapNames(reg)
			}
			src, err := shell.Generate(shellName, names)
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, src+"\n")
			return err
		},
	}
}

// wrapNames is the default command list: every shell utility, except battery
// on machines that have no battery.
func wrapNames(reg *command.Registry) []string {
	names := make([]string, 0, len(reg.List()))
	for _, e := range reg.List() {
		if e.Meta {
			continue
		}
		if e.Name == "battery" && !batteryPresent() {
			continue
		}
		names = append(names, e.Name)
	}
	return names
}

// initSetupText builds the copy-paste setup screen for the caller's shell.
func initSetupText() string {
	detected := shell.DetectShell(os.Getenv)
	var b strings.Builder
	b.WriteString("Usage:\n")
	b.WriteString("  incantations init [bash|zsh|fish] [command ...]\n")
	b.WriteString(`  eval "$(incantations init bash)"`)
	b.WriteString("\n\n")
	b.WriteString("Without arguments every utility is installed. Pass names to install\n")
	b.WriteString("only those, e.g. `init ram cpu`. On machines without a battery the\n")
	b.WriteString("battery wrapper is omitted automatically.\n\n")
	switch detected {
	case "bash", "zsh", "fish":
		fmt.Fprintf(&b, "Your shell looks like %s. Copy-paste this to install it:\n\n", detected)
		fmt.Fprintf(&b, "  %s\n\n", shell.SetupCommand(detected))
		fmt.Fprintf(&b, "That appends one line to %s so every new %s session gets the\nutilities. Re-running it is harmless.\n", shell.ConfigPath(detected), detected)
	default:
		b.WriteString("Couldn't tell which shell you use. Run one of these to install:\n\n")
		for _, s := range shell.Supported() {
			fmt.Fprintf(&b, "  %s\n", shell.SetupCommand(s))
		}
		b.WriteString("\nEach appends the integration line to that shell's config file.\n")
	}
	return b.String()
}
