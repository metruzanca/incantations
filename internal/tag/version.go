package tag

import (
	"strconv"
	"strings"
)

// bumpAt increments the run of digits touching pos by delta, resets the
// lower-order version segments to zero, and reports where the cursor should
// stay. It returns ok=false when pos is not on or beside a number.
func bumpAt(value string, pos, delta int) (string, int, bool) {
	if pos < 0 || pos > len(value) {
		return value, pos, false
	}

	digitIdx := -1
	switch {
	case pos > 0 && isDigit(value[pos-1]):
		digitIdx = pos - 1
	case pos < len(value) && isDigit(value[pos]):
		digitIdx = pos
	default:
		return value, pos, false
	}

	start := digitIdx
	for start > 0 && isDigit(value[start-1]) {
		start--
	}
	end := digitIdx + 1
	for end < len(value) && isDigit(value[end]) {
		end++
	}

	n, err := strconv.Atoi(value[start:end])
	if err != nil {
		return value, pos, false
	}

	segIdx := segmentIndex(value, start)

	n += delta
	if n < 0 {
		n = 0
	}

	width := end - start
	newNum := strconv.Itoa(n)
	if width > 1 && value[start] == '0' {
		for len(newNum) < width {
			newNum = "0" + newNum
		}
	}

	value = value[:start] + newNum + value[end:]
	value = resetLowerSegments(value, segIdx)
	return value, start + len(newNum), true
}

// segmentIndex returns the zero-based index of the version segment containing
// the byte at position start (major=0, minor=1, patch=2, ...). A leading "v"
// is ignored.
func segmentIndex(value string, start int) int {
	prefix := 0
	if strings.HasPrefix(value, "v") {
		prefix = 1
	}
	idx := 0
	for i := start - 1; i >= prefix; i-- {
		if value[i] == '.' {
			idx++
		}
	}
	return idx
}

// resetLowerSegments zeroes every segment after segIdx so that bumping a
// higher-order number also resets the lower-order ones (e.g. bumping the major
// clears the minor and patch).
func resetLowerSegments(value string, segIdx int) string {
	prefix := ""
	rest := value
	if strings.HasPrefix(rest, "v") {
		prefix = "v"
		rest = rest[1:]
	}
	segs := strings.Split(rest, ".")
	if segIdx < 0 || segIdx >= len(segs)-1 {
		return value
	}
	for i := segIdx + 1; i < len(segs); i++ {
		if allDigits(segs[i]) {
			segs[i] = "0"
		}
	}
	return prefix + strings.Join(segs, ".")
}

// latestVersion returns the highest version-shaped tag in tags, or "" when
// none of them look like a version.
func latestVersion(tags []string) string {
	best := ""
	var bestParts []int
	for _, t := range tags {
		parts, ok := parseVersion(t)
		if !ok {
			continue
		}
		if best == "" || compareVersionParts(parts, bestParts) > 0 {
			best = t
			bestParts = parts
		}
	}
	return best
}

// bumpMinor increments the minor component of a version tag and resets the
// lower components (e.g. v1.2.3 -> v1.3.0). It returns false when the tag has
// no minor component to bump.
func bumpMinor(version string) (string, bool) {
	groups := 0
	for i := 0; i < len(version); i++ {
		if isDigit(version[i]) {
			if groups == 1 {
				bumped, _, ok := bumpAt(version, i, 1)
				return bumped, ok
			}
			for i < len(version) && isDigit(version[i]) {
				i++
			}
			groups++
		}
	}
	return version, false
}

// parseVersion splits a version-shaped tag ("v1.2.3" or "1.2") into its numeric
// components. It requires at least two components, so a bare "v2" is not a
// version.
func parseVersion(t string) ([]int, bool) {
	s := t
	if strings.HasPrefix(s, "v") {
		s = s[1:]
	}
	if s == "" {
		return nil, false
	}
	segs := strings.Split(s, ".")
	if len(segs) < 2 {
		return nil, false
	}
	var parts []int
	for _, seg := range segs {
		if seg == "" {
			return nil, false
		}
		for _, c := range seg {
			if c < '0' || c > '9' {
				return nil, false
			}
		}
		n, err := strconv.Atoi(seg)
		if err != nil {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}

func compareVersionParts(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
