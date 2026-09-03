package net

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestParseDev(t *testing.T) {
	dev, err := parseDev(fixture(t, "dev.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := NetDev{
		"eth0": {RXBytes: 10000000, TXBytes: 5000000},
		"lo":   {RXBytes: 20000000, TXBytes: 20000000},
	}
	if !reflect.DeepEqual(dev, want) {
		t.Errorf("parseDev = %+v, want %+v", dev, want)
	}
}

func TestParseDevSkipsHeader(t *testing.T) {
	// A full-width row with the header block above it; only the row parses.
	dev, err := parseDev(strings.NewReader("Inter-|   Receive\n  face |bytes  packets\n  eth0:  1000  10  0  0  0  0  0  0  500  5  0  0  0  0  0  0\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dev) != 1 || dev["eth0"].RXBytes != 1000 || dev["eth0"].TXBytes != 500 {
		t.Errorf("parseDev = %+v", dev)
	}
}

func TestComputeRates(t *testing.T) {
	before := NetDev{
		"eth0":  {RXBytes: 1000, TXBytes: 500},
		"wlan0": {RXBytes: 9000, TXBytes: 3000},
	}
	after := NetDev{
		"eth0":  {RXBytes: 1100, TXBytes: 550},
		"wlan0": {RXBytes: 10000, TXBytes: 3100},
		"new0":  {RXBytes: 5, TXBytes: 5}, // missing from before -> dropped
	}
	rates := computeRates(before, after, time.Second)
	want := NetDev{
		"eth0":  {RXBytes: 100, TXBytes: 50},
		"wlan0": {RXBytes: 1000, TXBytes: 100},
	}
	if !reflect.DeepEqual(rates, want) {
		t.Errorf("computeRates = %+v, want %+v", rates, want)
	}
}

func TestComputeRatesCounterReset(t *testing.T) {
	before := NetDev{"eth0": {RXBytes: 9000, TXBytes: 9000}}
	after := NetDev{"eth0": {RXBytes: 100, TXBytes: 100}} // counters wrapped
	rates := computeRates(before, after, time.Second)
	if rates["eth0"].RXBytes != 0 || rates["eth0"].TXBytes != 0 {
		t.Errorf("wrapped counter should read 0, got %+v", rates["eth0"])
	}
}

func TestRender(t *testing.T) {
	rep := &Report{Ifaces: []Iface{
		{Name: "enp6s0", Addrs: []string{"192.168.1.151"}, Up: true, RX: 5_000_000, TX: 300_000},
		{Name: "lo", Addrs: []string{"127.0.0.1", "::1"}, Up: true, RX: 44_000, TX: 44_000},
		{Name: "wlp5s0", Up: true, RX: 12_500_000, TX: 2_000_000}, // no addresses -> "-"
		{Name: "tailscale0", Addrs: []string{"100.106.143.56"}, Up: false, RX: 0, TX: 0},
	}}
	golden(t, "net_render.golden", Render(rep, false, false))
}

func TestRenderIPv6(t *testing.T) {
	rep := &Report{Ifaces: []Iface{
		{Name: "wlp5s0", Addrs: []string{"192.168.1.151", "fe80::543b:c7cc:b376", "2600:4041::3af1"}, Up: true, RX: 1, TX: 1},
	}}
	defOut := Render(rep, false, false)
	if strings.Contains(defOut, "fe80:") {
		t.Error("IPv6 addresses should be hidden by default")
	}
	v6Out := Render(rep, false, true)
	for _, want := range []string{"192.168.1.151", "fe80::543b:c7cc:b376", "2600:4041::3af1"} {
		if !strings.Contains(v6Out, want) {
			t.Errorf("--ipv6 output missing %q:\n%s", want, v6Out)
		}
	}
}

func TestRenderAllIncludesLoopback(t *testing.T) {
	rep := &Report{Ifaces: []Iface{
		{Name: "lo", Addrs: []string{"127.0.0.1"}, Up: true, RX: 44_000, TX: 44_000},
	}}
	defaultOut := Render(rep, false, false)
	if strings.Contains(defaultOut, "lo") {
		t.Error("loopback should be hidden by default")
	}
	allOut := Render(rep, true, false)
	if !strings.Contains(allOut, "lo") {
		t.Error("-a should show loopback")
	}
}

func TestRenderEmpty(t *testing.T) {
	if got := Render(&Report{}, false, false); got != "No active network interfaces.\n" {
		t.Errorf("Render(empty) = %q", got)
	}
}

func TestRenderDeterministic(t *testing.T) {
	rep := &Report{Ifaces: []Iface{
		{Name: "wlan0", Up: true, RX: 1, TX: 1},
		{Name: "eth0", Up: true, RX: 2, TX: 2},
	}}
	if a, b := Render(rep, false, false), Render(rep, false, false); a != b {
		t.Error("Render must be deterministic")
	}
}
