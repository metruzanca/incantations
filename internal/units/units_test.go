package units

import "testing"

func TestHumanKiB(t *testing.T) {
	cases := map[uint64]string{
		0:        "0 KiB",
		1023:     "1023 KiB",
		1024:     "1.0 MiB",
		2048:     "2.0 MiB",
		1500:     "1.5 MiB",
		3145728:  "3.0 GiB",
		32746652: "31.2 GiB",
		1048576:  "1.0 GiB",
	}
	for in, want := range cases {
		if got := HumanKiB(in); got != want {
			t.Errorf("HumanKiB(%d) = %q, want %q", in, got, want)
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
