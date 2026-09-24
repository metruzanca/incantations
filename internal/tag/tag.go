// Package tag lists git tags and creates the next release tag. Running bare
// `incantations tag` opens a small TUI: the latest version is prefilled and the
// arrow keys bump its components, so cutting a release is a few key presses.
// `incantations tag v1.2.3` creates a tag directly for scripts and pipes.
package tag

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"golang.org/x/term"
)

// gitRun runs git and returns its stdout. It is a package var so tests stub it
// out (the same pattern as format.runFFmpeg and ports.signalProcess). git
// writes progress to stderr, which is kept out of the result; on failure the
// stderr tail is folded into the error so the user sees why.
var gitRun = func(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s: %s", err, lastLine(msg))
		}
		return "", err
	}
	return string(out), nil
}

// lastLine returns the final line of s, so a multi-line git error is not
// dragged into a one-line message.
func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// opts carries the parsed command-line flags for one invocation.
type opts struct {
	name string // tag to create; empty means open the TUI (or list when piped)
	push bool   // push the tag and its commits after creating it
}

// parseOpts parses an optional tag name and the --push flag.
func parseOpts(args []string) (opts, error) {
	var o opts
	for _, a := range args {
		switch {
		case a == "--push":
			o.push = true
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("unknown flag %q (usage: incantations tag [NAME] [--push])", a)
		case o.name != "":
			return o, fmt.Errorf("only one tag name at a time (got %q and %q)", o.name, a)
		default:
			o.name = a
		}
	}
	if o.push && o.name == "" {
		return o, fmt.Errorf("--push needs a tag name (usage: incantations tag NAME --push)")
	}
	return o, nil
}

// Spec registers the tag command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "tag",
		Summary: "list git tags and create the next one, bumping the version as you go",
		Help: `Usage:
  incantations tag              # open the tag TUI
  incantations tag NAME         # create tag NAME
  incantations tag NAME --push  # create it, then push it and its commits

With no arguments and a terminal, opens a small TUI listing the tags newest
version first. Press b to bump the latest version's minor number and create it
(a button, no prompt), or n to edit a new name. In the editor, up/down bump the
number under the cursor and reset the lower components, so bumping the major
clears the minor and patch. After a tag is created the TUI offers to push it
and its commits with one key.

Pass a tag name to skip the TUI entirely: the tag is created locally, and with
--push the current branch and the tag are pushed to origin. When piped, bare
tag prints the tags one per line instead of opening the TUI.`,
		Run: func(args []string, stdout io.Writer) error {
			return run(args, os.Stdin, stdout)
		},
	}
}

// run executes one tag invocation.
func run(args []string, stdin *os.File, stdout io.Writer) error {
	o, err := parseOpts(args)
	if err != nil {
		return err
	}
	if o.name != "" {
		return createTag(o.name, o.push, stdout)
	}
	if !term.IsTerminal(int(stdin.Fd())) {
		return listTags(stdout)
	}
	return runTUI()
}

// createTag creates name locally, optionally pushing it and its commits.
func createTag(name string, push bool, w io.Writer) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("tag name is required")
	}
	if err := gitCreateTag(name); err != nil {
		return fmt.Errorf("create tag %q: %w", name, err)
	}
	if !push {
		_, err := fmt.Fprintf(w, "Created tag %s. Push it with: git push origin %s\n", name, name)
		return err
	}
	if err := gitPushTagWithCommits(name); err != nil {
		return fmt.Errorf("push tag %q: %w", name, err)
	}
	_, err := fmt.Fprintf(w, "Pushed tag %s and its commits\n", name)
	return err
}

// listTags prints the tags, newest version first, one per line. Used when the
// output is piped rather than a terminal.
func listTags(w io.Writer) error {
	tags, err := gitTags()
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		_, err := io.WriteString(w, "No tags.\n")
		return err
	}
	_, err = io.WriteString(w, strings.Join(tags, "\n")+"\n")
	return err
}

// gitTags lists tags sorted so the newest version comes first and non-version
// tags follow alphabetically.
func gitTags() ([]string, error) {
	out, err := gitRun("tag", "--list")
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	var tags []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			tags = append(tags, line)
		}
	}
	return sortTags(tags), nil
}

// sortTags orders tags newest version first; tags that are not versions (e.g.
// "release") come after, alphabetically.
func sortTags(tags []string) []string {
	out := append([]string(nil), tags...)
	sort.SliceStable(out, func(i, j int) bool {
		pi, oki := parseVersion(out[i])
		pj, okj := parseVersion(out[j])
		switch {
		case oki && okj:
			if c := compareVersionParts(pi, pj); c != 0 {
				return c > 0
			}
			return out[i] > out[j]
		case oki:
			return true
		case okj:
			return false
		default:
			return out[i] < out[j]
		}
	})
	return out
}

func gitCreateTag(name string) error {
	_, err := gitRun("tag", name)
	return err
}

func gitPushTag(name string) error {
	_, err := gitRun("push", "origin", name)
	return err
}

// gitCurrentBranch returns the current branch name, or "HEAD" on a detached
// HEAD.
func gitCurrentBranch() (string, error) {
	out, err := gitRun("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitPushBranch(branch string) error {
	_, err := gitRun("push", "origin", branch)
	return err
}

// gitCommitOnRemote reports whether rev is reachable from any remote-tracking
// branch, i.e. whether its commits have been pushed.
func gitCommitOnRemote(rev string) (bool, error) {
	out, err := gitRun("branch", "-r", "--contains", rev)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// gitPushTagWithCommits pushes the commits a tag references before pushing the
// tag itself, so the remote never holds a tag for an unpushed commit. It
// pushes the current branch first; on a detached HEAD it verifies the tagged
// commit is already on a remote branch and refuses to push otherwise.
func gitPushTagWithCommits(name string) error {
	branch, err := gitCurrentBranch()
	if err != nil {
		return fmt.Errorf("resolve current branch: %w", err)
	}
	if branch != "HEAD" {
		if err := gitPushBranch(branch); err != nil {
			return fmt.Errorf("push branch %q: %w", branch, err)
		}
	} else {
		onRemote, err := gitCommitOnRemote(name)
		if err != nil {
			return fmt.Errorf("check remote for %q: %w", name, err)
		}
		if !onRemote {
			return fmt.Errorf("detached HEAD: commit referenced by tag is not on any remote branch, push it first")
		}
	}
	if err := gitPushTag(name); err != nil {
		return fmt.Errorf("push tag %q: %w", name, err)
	}
	return nil
}
