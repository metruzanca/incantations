package tag

import "testing"

func TestBumpAt(t *testing.T) {
	cases := []struct {
		value string
		pos   int
		delta int
		want  string
	}{
		{"v0.1.0", 3, 1, "v0.2.0"},
		{"v0.1.0", 4, 1, "v0.2.0"},
		{"v0.9.0", 3, 1, "v0.10.0"},
		{"v0.1.0", 2, 1, "v1.0.0"},
		{"v0.1.0", 1, 1, "v1.0.0"},
		{"v1.2.3", 1, 1, "v2.0.0"},
		{"v1.2.3", 3, 1, "v1.3.0"},
		{"v1.2.3", 5, 1, "v1.2.4"},
		{"v1.2.3", 2, -1, "v0.0.0"},
		{"v1.2.3", 3, -1, "v1.1.0"},
		{"v1.2.3", 5, -1, "v1.2.2"},
		{"1.2.3", 2, 1, "1.3.0"},
		{"v0.1.0", 5, 1, "v0.1.1"},
		{"v0.9.0", 5, 1, "v0.9.1"},
		{"v0.09.0", 3, 1, "v0.10.0"},
		{"v0.01.0", 3, 1, "v0.02.0"},
		{"v0.1.0", 4, -1, "v0.0.0"},
		{"v0.1.0", 3, -1, "v0.0.0"},
		{"v0.2.0", 3, -1, "v0.1.0"},
		{"v0.10.0", 3, -1, "v0.9.0"},
	}
	for _, c := range cases {
		got, _, ok := bumpAt(c.value, c.pos, c.delta)
		if !ok {
			t.Errorf("bumpAt(%q, %d, %d): unexpectedly failed", c.value, c.pos, c.delta)
			continue
		}
		if got != c.want {
			t.Errorf("bumpAt(%q, %d, %d) = %q, want %q", c.value, c.pos, c.delta, got, c.want)
		}
	}
}

func TestBumpAtFailsSilently(t *testing.T) {
	const value = "v0.1.0"
	got, _, ok := bumpAt(value, 0, 1)
	if ok {
		t.Errorf("bumpAt(%q, 0, 1): expected failure", value)
	}
	if got != value {
		t.Errorf("bumpAt(%q, 0, 1) = %q, want value unchanged", value, got)
	}
	if end, _, ok := bumpAt(value, len(value), 1); !ok || end != "v0.1.1" {
		t.Errorf("cursor at end: got %q ok=%v, want v0.1.1", end, ok)
	}

	nonNumeric := []string{"", "abc", "v..0"}
	badPos := []int{1, 1, 0}
	for i, v := range nonNumeric {
		got, _, ok := bumpAt(v, badPos[i], 1)
		if ok {
			t.Errorf("bumpAt(%q, %d, 1): expected failure", v, badPos[i])
		}
		if got != v {
			t.Errorf("bumpAt(%q, %d, 1): value changed to %q", v, badPos[i], got)
		}
	}
}

func TestBumpAtCursorSticks(t *testing.T) {
	got, pos, ok := bumpAt("v0.9.0", 4, 1)
	if !ok || got != "v0.10.0" {
		t.Fatalf("first bump: got %q ok=%v", got, ok)
	}
	got, _, ok = bumpAt(got, pos, 1)
	if !ok || got != "v0.11.0" {
		t.Fatalf("second bump from pos=%d: got %q ok=%v", pos, got, ok)
	}
}

func TestLatestVersion(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{[]string{"v1.9.0", "v1.10.0", "v1.2.3"}, "v1.10.0"},
		{[]string{"1.9.0", "1.10.0"}, "1.10.0"},
		{[]string{"v0.1.0", "v0.1.9", "v0.1.10"}, "v0.1.10"},
		{[]string{"v1.2.3", "release", "wip"}, "v1.2.3"},
		{[]string{}, ""},
		{[]string{"release", "foo"}, ""},
	}
	for _, c := range cases {
		if got := latestVersion(c.tags); got != c.want {
			t.Errorf("latestVersion(%v) = %q, want %q", c.tags, got, c.want)
		}
	}
}

func TestBumpMinor(t *testing.T) {
	cases := []struct {
		value string
		want  string
		ok    bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v1.2.3", "v1.3.0", true},
		{"v1.9.3", "v1.10.0", true},
		{"1.2.3", "1.3.0", true},
		{"v1.2", "v1.3", true},
		{"v0.0.0", "v0.1.0", true},
		{"v1", "", false},
		{"", "", false},
		{"v..0", "", false},
	}
	for _, c := range cases {
		got, ok := bumpMinor(c.value)
		if ok != c.ok {
			t.Errorf("bumpMinor(%q) ok=%v, want %v", c.value, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("bumpMinor(%q) = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestSortTags(t *testing.T) {
	got := sortTags([]string{"v1.2.3", "release", "v1.10.0", "wip", "v1.9.0", "v0.1.0"})
	want := []string{"v1.10.0", "v1.9.0", "v1.2.3", "v0.1.0", "release", "wip"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortTags = %v, want %v", got, want)
		}
	}
}
