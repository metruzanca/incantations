package units

import "testing"

func TestHumanMemory(t *testing.T) {
	cases := map[uint64]string{
		0:          "0 KB",
		1023:       "1.0 MB",
		1024:       "1.0 MB",
		1500:       "1.5 MB",
		2048:       "2.1 MB",
		1048576:    "1.1 GB",
		3145728:    "3.2 GB",
		5242880:    "5.4 GB",
		1061683200: "1.1 TB",
	}
	for in, want := range cases {
		if got := HumanMemory(in); got != want {
			t.Errorf("HumanMemory(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestPct(t *testing.T) {
	if got := Pct(25, 100); got != 25.0 {
		t.Errorf("Pct(25,100) = %v", got)
	}
	if got := Pct(5, 0); got != 0 {
		t.Errorf("Pct with zero whole = %v, want 0", got)
	}
}
