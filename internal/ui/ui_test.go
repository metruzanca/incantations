package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestProgressBarDeterministic(t *testing.T) {
	a := ProgressBar(0.5, 20)
	b := ProgressBar(0.5, 20)
	if a != b {
		t.Fatal("progress bar must be deterministic")
	}
	if got := len([]rune(a)); got != 20 {
		t.Errorf("bar width = %d, want 20", got)
	}
}

func TestProgressBarClamps(t *testing.T) {
	for _, ratio := range []float64{5, -1} {
		if got := ProgressBar(ratio, 10); len([]rune(got)) != 10 {
			t.Errorf("ratio %v should clamp to the bar: %q", ratio, got)
		}
	}
}

func TestProgressBarEmptyAndFull(t *testing.T) {
	empty := ProgressBar(0, 10)
	if strings.Trim(empty, "░") != "" {
		t.Errorf("empty bar should be all '░': %q", empty)
	}
	full := ProgressBar(1, 10)
	if strings.Trim(full, "█") != "" {
		t.Errorf("full bar should be all '█': %q", full)
	}
}

func TestProgressBarNeverEmitsEscapes(t *testing.T) {
	if strings.Contains(ProgressBar(0.5, 10), "\x1b") {
		t.Error("ProgressBar inside table cells must never contain ANSI escapes")
	}
	if strings.Contains(Bar(0.5, 10), "\x1b") {
		t.Error("plain Bar must not contain escapes when unstyled")
	}
}

func TestNewTableMultibyteWidths(t *testing.T) {
	// Block characters are 3 bytes but one terminal cell; a byte-based width
	// would over-inflate the column and wrap the row.
	out := NewTable(
		[]string{"USAGE", "MOUNT"},
		[]bool{false, false},
		[][]string{{"████░░░ 24%", "/"}},
	)
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly 2 lines, got %d:\n%q", len(lines), out)
	}
	for _, line := range lines {
		if ansi.StringWidth(line) > 40 {
			t.Errorf("row way too wide (%d cells): %q", ansi.StringWidth(line), line)
		}
	}
}
