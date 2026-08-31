package ui

import (
	"strings"
	"testing"
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
