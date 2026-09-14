package format

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// pick shows an interactive dropdown of the target formats and returns the one
// the user picks. It renders to the terminal (stderr) so stdout stays
// pipeable. The caller ensures stdin is a terminal before calling.
func pick(targets []string, file string) (string, error) {
	m := pickerModel{targets: targets, file: file}
	p := tea.NewProgram(m, tea.WithInput(os.Stdin), tea.WithOutput(os.Stderr))
	model, err := p.Run()
	if err != nil {
		return "", err
	}
	return model.(pickerModel).chosen, nil
}

// pickerModel is a minimal single-select list: up/down moves, enter picks,
// q/ctrl+c quits without converting. It is kept free of terminal state so the
// Update/View logic can be unit-tested without a TTY.
type pickerModel struct {
	targets []string
	cursor  int
	chosen  string
	file    string
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyUp:
		m.up()
	case tea.KeyDown:
		m.down()
	case tea.KeyEnter:
		m.chosen = m.targets[m.cursor]
		return m, tea.Quit
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "q":
			return m, tea.Quit
		case "k":
			m.up()
		case "j":
			m.down()
		}
	}
	return m, nil
}

func (m *pickerModel) up() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *pickerModel) down() {
	if m.cursor < len(m.targets)-1 {
		m.cursor++
	}
}

func (m pickerModel) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Convert %s to:\n\n", m.file)
	for i, t := range m.targets {
		marker := " "
		if i == m.cursor {
			marker = ">"
		}
		fmt.Fprintf(&b, "%s .%s\n", marker, t)
	}
	fmt.Fprintf(&b, "\n\u2191/\u2193 choose \u00b7 enter convert \u00b7 q quit\n")
	return b.String()
}