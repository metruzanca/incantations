package format

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newPicker() pickerModel {
	return pickerModel{targets: []string{"gif", "mp4", "mkv"}, file: "clip.webm"}
}

func TestPickerInitialView(t *testing.T) {
	m := newPicker()
	v := m.View()
	for _, want := range []string{"Convert clip.webm to:", ".gif", ".mp4", ".mkv"} {
		if !strings.Contains(v, want) {
			t.Errorf("picker view missing %q:\n%s", want, v)
		}
	}
	if !strings.Contains(v, "> .gif") {
		t.Errorf("first target should be selected:\n%s", v)
	}
}

func step(t *testing.T, m tea.Model, msg tea.KeyMsg) pickerModel {
	t.Helper()
	m, _ = m.Update(msg)
	return m.(pickerModel)
}

func TestPickerNavigation(t *testing.T) {
	m := newPicker()

	m = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("down should move to index 1, got %d", m.cursor)
	}
	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 {
		t.Errorf("j should move to index 2, got %d", m.cursor)
	}
	// At the last target, down and j clamp.
	m = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("down should clamp at last index, got %d", m.cursor)
	}
	// Up and k move back up; up at index 0 clamps.
	m = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Errorf("up should clamp at index 0, got %d", m.cursor)
	}
}

func TestPickerSelect(t *testing.T) {
	m := step(t, newPicker(), tea.KeyMsg{Type: tea.KeyDown})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.chosen != "mp4" {
		t.Errorf("enter should pick the cursor target, got %q", m.chosen)
	}
}

func TestPickerQuit(t *testing.T) {
	m, cmd := newPicker().Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.(pickerModel).chosen != "" {
		t.Error("quit must not choose a target")
	}
	if cmd == nil {
		t.Error("quit should return a quit command")
	}
}