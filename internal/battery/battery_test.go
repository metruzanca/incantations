package battery

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden testdata files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	switch {
	case err != nil && !*update:
		t.Fatalf("reading golden %s: %v (regenerate with -update)", path, err)
	case *update:
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		t.Logf("updated %s", path)
	case string(want) != got:
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

func fixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestParseUevent(t *testing.T) {
	b, err := parseUevent(fixture(t, "uevent.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := Battery{
		Status:       "Discharging",
		Percent:      40,
		EnergyNow:    37810000,
		EnergyFull:   79300000,
		EnergyDesign: 90000000,
		PowerNow:     22045528,
	}
	if b != want {
		t.Errorf("parseUevent = %+v, want %+v", b, want)
	}
}

func TestParseUeventChargeFallback(t *testing.T) {
	// Batteries that expose CHARGE_* (µAh) instead of ENERGY_* (µWh) get
	// converted using the current voltage, and CURRENT_NOW × voltage backs
	// the power figure.
	src := "POWER_SUPPLY_STATUS=Charging\n" +
		"POWER_SUPPLY_CAPACITY=50\n" +
		"POWER_SUPPLY_VOLTAGE_NOW=12008000\n" +
		"POWER_SUPPLY_CURRENT_NOW=2000000\n" +
		"POWER_SUPPLY_CHARGE_NOW=4000000\n" +
		"POWER_SUPPLY_CHARGE_FULL=8000000\n" +
		"POWER_SUPPLY_CHARGE_FULL_DESIGN=8500000\n"
	b, err := parseUevent(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	want := Battery{
		Status:       "Charging",
		Percent:      50,
		EnergyNow:    4000000 * 12008000 / 1e6,
		EnergyFull:   8000000 * 12008000 / 1e6,
		EnergyDesign: 8500000 * 12008000 / 1e6,
		PowerNow:     2000000 * 12008000 / 1e6,
	}
	if b != want {
		t.Errorf("parseUevent = %+v, want %+v", b, want)
	}
}

func TestRemaining(t *testing.T) {
	cases := []struct {
		status string
		now    uint64
		full   uint64
		power  uint64
		want   int64
	}{
		{"Discharging", 1_000_000, 2_000_000, 1_000_000, 3600}, // 1 Wh at 1 W
		{"Charging", 1_000_000, 2_000_000, 1_000_000, 3600},    // 1 Wh gap at 1 W
		{"Full", 1_000_000, 2_000_000, 1_000_000, 0},
		{"Discharging", 1_000_000, 2_000_000, 0, 0},
	}
	for _, tc := range cases {
		b := Battery{Status: tc.status, EnergyNow: tc.now, EnergyFull: tc.full, PowerNow: tc.power}
		if got := remaining(b); got != tc.want {
			t.Errorf("remaining(%s) = %d, want %d", tc.status, got, tc.want)
		}
	}
}

func TestRender(t *testing.T) {
	b, err := parseUevent(fixture(t, "uevent.txt"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "battery_render.golden", Render(&Report{Found: true, B: b}))
}

func TestRenderNoBattery(t *testing.T) {
	if got := Render(&Report{}); got != "No battery found.\n" {
		t.Errorf("Render(no battery) = %q", got)
	}
}

func TestRenderDeterministic(t *testing.T) {
	b, err := parseUevent(fixture(t, "uevent.txt"))
	if err != nil {
		t.Fatal(err)
	}
	rep := &Report{Found: true, B: b}
	if a, b := Render(rep), Render(rep); a != b {
		t.Error("Render must be deterministic")
	}
}

// TestSample points Sample's powerRoot at a temporary tree so the platform-
// specific discovery logic is exercised without real hardware.
func TestSample(t *testing.T) {
	root := t.TempDir()
	writeFile := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A charger must be skipped; BAT0 carries the battery.
	writeFile("AC/type", "Mains\n")
	uv, err := os.ReadFile(filepath.Join("testdata", "uevent.txt"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile("BAT0/type", "Battery\n")
	writeFile("BAT0/uevent", string(uv))

	old := powerRoot
	powerRoot = root
	defer func() { powerRoot = old }()

	rep, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Found {
		t.Fatal("expected a battery to be found")
	}
	if rep.B.Status != "Discharging" || rep.B.Percent != 40 {
		t.Errorf("unexpected battery: %+v", rep.B)
	}
}

func TestSampleNoBattery(t *testing.T) {
	old := powerRoot
	powerRoot = t.TempDir() // empty: no power supplies at all
	defer func() { powerRoot = old }()

	rep, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Found {
		t.Error("Found should be false with no battery present")
	}
}
