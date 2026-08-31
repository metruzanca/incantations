// Package shell generates deterministic shell functions (bash, zsh, fish) that
// wrap the incantations binary so users get plain `ram`, `cpu`, `disk` etc.
// commands. The output is stable for a given command list, so re-evaluating
// init output is idempotent.
package shell

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Generate returns shell source that defines a function for each command in
// the given list. Running eval over this source is safe and idempotent.
func Generate(shell string, commands []string) (string, error) {
	name := normalize(shell)
	commands = append([]string(nil), commands...)
	sort.Strings(commands)
	switch name {
	case "bash":
		return shIntegration("bash", commands, `"$@"`), nil
	case "zsh":
		return shIntegration("zsh", commands, `"$@"`), nil
	case "fish":
		return fishIntegration(commands), nil
	}
	return "", fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", shell)
}

// Sh supported shells for error messages.
func Supported() []string {
	return []string{"bash", "zsh", "fish"}
}

func normalize(shell string) string {
	s := strings.ToLower(strings.TrimSpace(shell))
	s = strings.Trim(s, "/")
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// DetectShell guesses the caller's interactive shell from the SHELL env var
// (or shell-specific version variables), returning "bash", "zsh", "fish", or
// "" when it cannot tell. The env lookup is injected so tests stay hermetic.
func DetectShell(env func(string) string) string {
	base := filepath.Base(env("SHELL"))
	switch {
	case strings.Contains(base, "zsh"):
		return "zsh"
	case strings.Contains(base, "fish"):
		return "fish"
	case strings.Contains(base, "bash"), base == "sh":
		return "bash"
	}
	switch {
	case env("ZSH_VERSION") != "":
		return "zsh"
	case env("BASH_VERSION") != "":
		return "bash"
	}
	return ""
}

// ConfigPath returns the shell configuration file (as a display path).
func ConfigPath(shell string) string {
	switch shell {
	case "zsh":
		return "~/.zshrc"
	case "fish":
		return "~/.config/fish/config.fish"
	default:
		return "~/.bashrc"
	}
}

// SetupCommand returns a copy-paste command that appends the integration line
// to the shell's config file, e.g.
// echo 'eval "$(incantations init bash)"' >> ~/.bashrc
func SetupCommand(shell string) string {
	switch shell {
	case "zsh":
		return `echo 'eval "$(incantations init zsh)"' >> ~/.zshrc`
	case "fish":
		return "echo 'incantations init fish | source' >> ~/.config/fish/config.fish"
	default:
		return `echo 'eval "$(incantations init bash)"' >> ~/.bashrc`
	}
}

// shIntegration covers bash and zsh, whose function syntax is identical.
func shIntegration(shell string, commands []string, dispatch string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# incantations init %s (generated, do not edit)\n", shell)
	for i, name := range commands {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s() {\n", name)
		fmt.Fprintf(&b, "    incantations %s %s\n", name, dispatch)
		b.WriteString("}\n")
	}
	return b.String()
}

func fishIntegration(commands []string) string {
	var b strings.Builder
	b.WriteString("# incantations init fish (generated, do not edit)\n")
	for i, name := range commands {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "function %s\n", name)
		fmt.Fprintf(&b, "    incantations %s $argv\n", name)
		b.WriteString("end\n")
	}
	return b.String()
}
