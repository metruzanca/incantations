// Package command defines the CLI command registry shared by dispatch and
// shell integration generation.
package command

import (
	"io"
	"sort"
)

// Run executes a command with the remaining arguments, writing human-readable
// output to stdout.
type Run func(args []string, stdout io.Writer) error

// Entry is a single registered subcommand.
type Entry struct {
	Name    string
	Summary string // one-line description shown in help
	// Meta marks shell-utility commands as false and control commands (like
	// init) as true. Only non-meta commands get generated shell functions.
	Meta bool
	Run  Run
}

// Registry holds the ordered set of commands for this binary.
type Registry struct {
	byName map[string]Entry
}

func New() *Registry {
	return &Registry{byName: make(map[string]Entry)}
}

func (r *Registry) Add(e Entry) {
	r.byName[e.Name] = e
}

func (r *Registry) Get(name string) (Entry, bool) {
	e, ok := r.byName[name]
	return e, ok
}

// List returns the registered commands sorted by name. Sorting keeps help
// output and generated shell functions deterministic.
func (r *Registry) List() []Entry {
	entries := make([]Entry, 0, len(r.byName))
	for _, e := range r.byName {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}
