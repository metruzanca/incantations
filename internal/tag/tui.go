package tag

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// mode is which screen the tag TUI is showing.
type mode int

const (
	modeList mode = iota // browse tags and pick an action
	modeEdit             // type or bump a new tag name
	modePush             // offer to push the tag just created
)

// model is the tag TUI. It is kept free of terminal state so Update and View
// can be unit-tested without a TTY, like the format picker.
type model struct {
	tags   []string
	cursor int
	err    error
	notice string

	mode    mode
	input   textinput.Model
	latest  string
	tagErr  string
	pending string // tag created and waiting on the push offer
}

func newModel() *model {
	m := &model{}
	m.refresh()
	return m
}

func (m *model) Init() tea.Cmd { return nil }

// refresh reloads the tag list from git and re-clamps the cursor.
func (m *model) refresh() {
	tags, err := gitTags()
	if err != nil {
		m.err = err
		m.tags = nil
		return
	}
	m.err = nil
	m.tags = tags
	if m.cursor >= len(m.tags) {
		m.cursor = max(0, len(m.tags)-1)
	}
}

// openEdit prefills the editor with the latest version tag, or v0.1.0 when
// there is none.
func (m *model) openEdit() {
	latest := latestVersion(m.tags)
	m.latest = latest
	start := latest
	if start == "" {
		start = "v0.1.0"
	}
	ti := textinput.New()
	ti.Placeholder = "tag name (e.g. v0.1.0)"
	ti.Width = 40
	ti.SetValue(start)
	ti.SetCursor(len(start))
	m.input = ti
	m.tagErr = ""
	m.mode = modeEdit
}

// beginBump is the "bump" button: it bumps the latest version's minor number
// and creates the tag in one step, with no extra prompt.
func (m *model) beginBump() {
	m.tagErr = ""
	if m.err != nil {
		return
	}
	latest := latestVersion(m.tags)
	if latest == "" {
		m.notice = "no versioned tags to bump; press n to name one"
		return
	}
	name, ok := bumpMinor(latest)
	if !ok {
		m.notice = fmt.Sprintf("could not bump %q", latest)
		return
	}
	if err := gitCreateTag(name); err != nil {
		m.notice = fmt.Sprintf("tag failed: %v", err)
		return
	}
	m.pending = name
	m.mode = modePush
	m.refresh()
}

// createTag commits the editor's value as a tag and moves to the push offer.
func (m *model) createTag() {
	name := strings.TrimSpace(m.input.Value())
	if name == "" {
		m.tagErr = "tag name is required"
		return
	}
	if err := gitCreateTag(name); err != nil {
		m.tagErr = fmt.Sprintf("tag failed: %v", err)
		return
	}
	m.pending = name
	m.mode = modePush
	m.refresh()
}

// resolvePush finishes the push offer.
func (m *model) resolvePush(push bool) {
	name := m.pending
	m.pending = ""
	if push {
		if err := gitPushTagWithCommits(name); err != nil {
			m.notice = fmt.Sprintf("created %s, but push failed: %v", name, err)
		} else {
			m.notice = fmt.Sprintf("pushed %s and its commits", name)
		}
	} else {
		m.notice = fmt.Sprintf("created %s (not pushed)", name)
	}
	m.mode = modeList
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.mode {
	case modeEdit:
		return m.updateEdit(key)
	case modePush:
		return m.updatePush(key)
	default:
		return m.updateList(key)
	}
}

func (m *model) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.tags)-1 {
			m.cursor++
		}
	case "n":
		m.openEdit()
		return m, m.input.Focus()
	case "b":
		m.beginBump()
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) updateEdit(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "ctrl+c":
		m.mode = modeList
		return m, nil
	case "enter":
		m.createTag()
		return m, nil
	case "up":
		m.bumpCursor(1)
		return m, nil
	case "down":
		m.bumpCursor(-1)
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

// bumpCursor bumps the number under the text cursor and keeps the cursor in
// place.
func (m *model) bumpCursor(delta int) {
	value, pos, ok := bumpAt(m.input.Value(), m.input.Position(), delta)
	if !ok {
		return
	}
	m.input.SetValue(value)
	m.input.SetCursor(pos)
}

func (m *model) updatePush(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "enter":
		m.resolvePush(true)
	case "n", "esc", "q", "ctrl+c":
		m.resolvePush(false)
	}
	return m, nil
}

func (m *model) View() string {
	switch m.mode {
	case modeEdit:
		return m.editView()
	case modePush:
		return m.pushView()
	default:
		return m.listView()
	}
}

func (m *model) listView() string {
	var b strings.Builder
	b.WriteString("Tags\n\n")

	switch {
	case m.err != nil:
		fmt.Fprintf(&b, "  error: %v\n", m.err)
	case len(m.tags) == 0:
		b.WriteString("  (no tags)\n")
	default:
		for i, t := range m.tags {
			marker := "  "
			if i == m.cursor {
				marker = "> "
			}
			fmt.Fprintf(&b, "%s%s\n", marker, t)
		}
	}

	if m.notice != "" {
		fmt.Fprintf(&b, "\n  %s\n", m.notice)
	}

	footer := "↑/↓ move · n new tag · q quit"
	if latest := latestVersion(m.tags); latest != "" {
		if next, ok := bumpMinor(latest); ok {
			footer = fmt.Sprintf("b bump to %s · %s", next, footer)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", footer)
	return b.String()
}

func (m *model) editView() string {
	var b strings.Builder
	b.WriteString("New tag\n\n")

	latest := m.latest
	if latest == "" {
		latest = "(none)"
	}
	fmt.Fprintf(&b, "Latest: %s\n\n", latest)
	fmt.Fprintf(&b, "Name:\n%s\n", m.input.View())

	if m.tagErr != "" {
		fmt.Fprintf(&b, "\n  %s\n", m.tagErr)
	}
	b.WriteString("\n←/→ move · ↑/↓ bump number · enter create · esc cancel\n")
	return b.String()
}

func (m *model) pushView() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Created %s.\n\n", m.pending)
	b.WriteString("Push it and its commits to origin?\n\n")
	b.WriteString("y push · n skip\n")
	return b.String()
}

// runTUI opens the tag TUI on the terminal.
func runTUI() error {
	p := tea.NewProgram(newModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
