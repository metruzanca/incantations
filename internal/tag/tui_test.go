package tag

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel builds a model with a fixed tag list, bypassing git.
func newTestModel(t *testing.T, tags []string) *model {
	t.Helper()
	fakeGit(t, func(args ...string) (string, error) {
		if len(args) > 0 && args[0] == "tag" && len(args) == 2 && args[1] == "--list" {
			return strings.Join(tags, "\n") + "\n", nil
		}
		return "", nil
	})
	return newModel()
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func update(t *testing.T, m *model, k string) *model {
	t.Helper()
	mm, _ := m.Update(key(k))
	return mm.(*model)
}

func TestListViewShowsSortedTags(t *testing.T) {
	m := newTestModel(t, []string{"v0.1.0", "v1.2.3", "v1.10.0"})
	v := m.View()
	for _, want := range []string{"Tags", "v1.10.0", "v1.2.3", "v0.1.0", "> v1.10.0"} {
		if !strings.Contains(v, want) {
			t.Errorf("list view missing %q:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "b bump to v1.11.0") {
		t.Errorf("list view should advertise the bump button:\n%s", v)
	}
}

func TestListNavigationClamps(t *testing.T) {
	m := newTestModel(t, []string{"v0.1.0", "v0.2.0"})
	m = update(t, m, "down")
	if m.cursor != 1 {
		t.Errorf("down -> cursor %d, want 1", m.cursor)
	}
	m = update(t, m, "down")
	if m.cursor != 1 {
		t.Errorf("down should clamp, cursor %d", m.cursor)
	}
	m = update(t, m, "k")
	m = update(t, m, "k")
	if m.cursor != 0 {
		t.Errorf("up should clamp at 0, cursor %d", m.cursor)
	}
}

// A b press creates the bumped minor tag immediately, then offers the push.
func TestBumpButtonCreatesTagWithoutPrompt(t *testing.T) {
	var created string
	calls := &[]string{}
	old := gitRun
	gitRun = func(args ...string) (string, error) {
		*calls = append(*calls, strings.Join(args, " "))
		switch strings.Join(args, " ") {
		case "tag --list":
			return "v1.2.3\n", nil
		case "tag v1.3.0":
			created = "v1.3.0"
		}
		return "", nil
	}
	t.Cleanup(func() { gitRun = old })

	m := newModel()
	m = update(t, m, "b")
	if created != "v1.3.0" {
		t.Fatalf("b should create v1.3.0, created %q (calls: %v)", created, *calls)
	}
	if m.mode != modePush {
		t.Fatalf("b should move straight to the push offer, mode %d", m.mode)
	}
	if !strings.Contains(m.View(), "Created v1.3.0") {
		t.Errorf("push view = %q", m.View())
	}
}

func TestPushOfferResolves(t *testing.T) {
	calls := &[]string{}
	old := gitRun
	gitRun = func(args ...string) (string, error) {
		*calls = append(*calls, strings.Join(args, " "))
		switch strings.Join(args, " ") {
		case "tag --list":
			return "v1.2.3\n", nil
		case "rev-parse --abbrev-ref HEAD":
			return "main\n", nil
		}
		return "", nil
	}
	t.Cleanup(func() { gitRun = old })

	m := newModel()
	m = update(t, m, "b")
	m = update(t, m, "y")
	if m.mode != modeList {
		t.Fatalf("after y the model should return to the list, mode %d", m.mode)
	}
	joined := strings.Join(*calls, "|")
	if !strings.Contains(joined, "push origin main") || !strings.Contains(joined, "push origin v1.3.0") {
		t.Errorf("push should push branch then tag, calls: %s", joined)
	}
	if !strings.Contains(m.notice, "pushed v1.3.0") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestPushOfferSkip(t *testing.T) {
	m := newTestModel(t, []string{"v1.2.3"})
	m = update(t, m, "b")
	m = update(t, m, "n")
	if m.mode != modeList {
		t.Fatalf("n should return to the list, mode %d", m.mode)
	}
	if !strings.Contains(m.notice, "not pushed") {
		t.Errorf("notice = %q", m.notice)
	}
}

func TestEditPrefillsLatestAndCreates(t *testing.T) {
	var created string
	old := gitRun
	gitRun = func(args ...string) (string, error) {
		if strings.Join(args, " ") == "tag --list" {
			return "v1.2.3\n", nil
		}
		created = strings.Join(args, " ")
		return "", nil
	}
	t.Cleanup(func() { gitRun = old })

	m := newModel()
	m = update(t, m, "n")
	if m.mode != modeEdit {
		t.Fatalf("n should open the editor, mode %d", m.mode)
	}
	if got := m.input.Value(); got != "v1.2.3" {
		t.Fatalf("editor should prefill the latest tag, got %q", got)
	}
	m.input.SetCursor(len(m.input.Value()))
	m = update(t, m, "up") // bump patch: v1.2.3 -> v1.2.4
	if got := m.input.Value(); got != "v1.2.4" {
		t.Fatalf("up should bump the patch, got %q", got)
	}
	m = update(t, m, "enter")
	if created != "tag v1.2.4" {
		t.Fatalf("enter should create v1.2.4, created %q", created)
	}
	if m.mode != modePush {
		t.Fatalf("creating should offer the push, mode %d", m.mode)
	}
}

func TestEditEmptyNameRejected(t *testing.T) {
	m := newTestModel(t, []string{"v1.2.3"})
	m = update(t, m, "n")
	m.input.SetValue("")
	m = update(t, m, "enter")
	if m.mode != modeEdit {
		t.Fatalf("an empty name must stay in the editor, mode %d", m.mode)
	}
	if !strings.Contains(m.View(), "required") {
		t.Errorf("expected a required-name error:\n%s", m.View())
	}
}

func TestBumpButtonWithNoVersions(t *testing.T) {
	m := newTestModel(t, []string{"release", "wip"})
	m = update(t, m, "b")
	if m.mode != modeList {
		t.Fatalf("nothing to bump should stay on the list, mode %d", m.mode)
	}
	if !strings.Contains(m.notice, "no versioned tags") {
		t.Errorf("notice = %q", m.notice)
	}
}
