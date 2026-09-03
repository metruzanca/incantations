package ports

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

func TestParseSs(t *testing.T) {
	rows, err := parseSs(fixture(t, "ss.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("parsed %d rows, want 10", len(rows))
	}
	if rows[0].Proto != "udp" {
		t.Errorf("first row proto = %q, want udp", rows[0].Proto)
	}
	// Process and PID extraction from the users:(...) column.
	if rows[0].Process != "brave" || rows[0].PID != 86298 {
		t.Errorf("row 0 process = %q pid=%d", rows[0].Process, rows[0].PID)
	}
	// Dotted process names must survive intact.
	if rows[3].Process != ".spotify-wrappe" || rows[3].PID != 3195 {
		t.Errorf("row 3 process = %q pid=%d", rows[3].Process, rows[3].PID)
	}
	// A socket without a process column leaves both blank.
	if rows[6].Local != "0.0.0.0:22" || rows[6].Process != "" || rows[6].PID != 0 {
		t.Errorf("row 6 = %+v", rows[6])
	}
	// IPv6 zone-qualified addresses are preserved.
	if rows[1].Local != "[fe80::543b:893b:c7cc:b376]%wlp5s0:546" {
		t.Errorf("row 1 local = %q", rows[1].Local)
	}
}

func TestParseSsSkipsHeaderAndWrapped(t *testing.T) {
	src := "Netid State  Recv-Q Send-Q  Local Address:Port  Peer Address:PortProcess\n" +
		"   some wrapped continuation line without a netid\n" +
		"tcp   LISTEN 0      128      0.0.0.0:22   0.0.0.0:*\n"
	rows, err := parseSs(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Local != "0.0.0.0:22" {
		t.Fatalf("rows = %+v, want just the tcp row", rows)
	}
}

func TestParseUsersMultiple(t *testing.T) {
	// ss can report several processes per socket; the first one wins.
	name, pid := parseUsers([]string{`users:(("first",pid=1,fd=1),("second",pid=2,fd=2))`})
	if name != "first" || pid != 1 {
		t.Errorf("parseUsers = %q %d, want first 1", name, pid)
	}
}

func TestParseServices(t *testing.T) {
	m := make(map[int]string)
	parseServices(fixture(t, "services.txt"), m)
	if m[22] != "ssh" || m[631] != "ipp" || m[80] != "http" {
		t.Errorf("services = %v", m)
	}
	// First name wins: 53 is listed for both tcp and udp as "domain".
	if m[53] != "domain" {
		t.Errorf("services[53] = %q, want domain", m[53])
	}
	if _, ok := m[9999]; ok {
		t.Error("unknown port must not be present")
	}
}

func TestLocalPort(t *testing.T) {
	cases := map[string]int{
		"0.0.0.0:22":           22,
		"[::1]:631":            631,
		"*:60731":              60731,
		"[fe80::1]%wlp5s0:546": 546,
		"nothing":              0,
	}
	for in, want := range cases {
		if got := localPort(in); got != want {
			t.Errorf("localPort(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestRender(t *testing.T) {
	rows, err := parseSs(fixture(t, "ss.txt"))
	if err != nil {
		t.Fatal(err)
	}
	svc := make(map[int]string)
	parseServices(fixture(t, "services.txt"), svc)
	// Default view: TCP over IPv4, grouped by process.
	golden(t, "ports_render.golden", Render(&Report{Rows: rows, Svc: svc}, false, false))
}

func TestRenderEmpty(t *testing.T) {
	if got := Render(&Report{}, false, false); got != "No listening ports found.\n" {
		t.Errorf("Render(empty) = %q", got)
	}
}

func TestRenderDeterministic(t *testing.T) {
	rows, err := parseSs(fixture(t, "ss.txt"))
	if err != nil {
		t.Fatal(err)
	}
	rep := &Report{Rows: rows}
	if a, b := Render(rep, false, true), Render(rep, false, true); a != b {
		t.Error("Render must be deterministic")
	}
}

func TestRenderGroupsByPID(t *testing.T) {
	rows, err := parseSs(fixture(t, "ss.txt"))
	if err != nil {
		t.Fatal(err)
	}
	svc := make(map[int]string)
	parseServices(fixture(t, "services.txt"), svc)
	// TCP v4 rows only, one table: chromium owns 46747, unowned gets 22 + 631.
	got := Render(&Report{Rows: rows, Svc: svc}, false, false)
	for _, want := range []string{"PROCESS", "chromium", "1228742", "127.0.0.1:46747", "0.0.0.0:22 (ssh)"} {
		if !strings.Contains(got, want) {
			t.Errorf("grouped output missing %q:\n%s", want, got)
		}
	}
	// A continuation socket must not print its process/PID again.
	if strings.Count(got, "chromium") != 1 || strings.Count(got, "1228742") != 1 {
		t.Errorf("process/PID should appear once per group:\n%s", got)
	}
	if strings.Contains(got, ".spotify-wrappe") && strings.Contains(got, "5353") {
		t.Error("UDP sockets should be hidden by default")
	}
}

func TestRenderUDPRequiresFlag(t *testing.T) {
	rows, err := parseSs(fixture(t, "ss.txt"))
	if err != nil {
		t.Fatal(err)
	}
	def := Render(&Report{Rows: rows}, false, false)
	if strings.Contains(def, "udp") {
		t.Error("UDP sockets should be hidden unless --udp")
	}
	// Without --udp everything shown is TCP, so PROTO is redundant.
	if strings.Contains(def, "PROTO") || strings.Contains(def, "tcp ") {
		t.Errorf("PROTO column should be dropped without --udp:\n%s", def)
	}
	withUDP := Render(&Report{Rows: rows}, false, true)
	if !strings.Contains(withUDP, "udp") {
		t.Error("--udp should include UDP sockets")
	}
	if !strings.Contains(withUDP, "PROTO") {
		t.Error("--udp should keep the PROTO column")
	}
}

func TestRenderIPv6RequiresFlag(t *testing.T) {
	rows, err := parseSs(fixture(t, "ss.txt"))
	if err != nil {
		t.Fatal(err)
	}
	def := Render(&Report{Rows: rows}, false, false)
	if strings.Contains(def, "[::") || strings.Contains(def, "tcp6") {
		t.Error("IPv6 sockets should be hidden unless --ipv6")
	}
	withV6 := Render(&Report{Rows: rows}, true, false)
	if !strings.Contains(withV6, "[::") {
		t.Error("--ipv6 should include IPv6 sockets")
	}
}

func TestParseOpts(t *testing.T) {
	cases := []struct {
		args    []string
		want    opts
		wantErr bool
	}{
		{nil, opts{}, false},
		{[]string{"8080"}, opts{filter: 8080}, false},
		{[]string{"--udp", "--ipv6", "22"}, opts{filter: 22, showV6: true, showUDP: true}, false},
		{[]string{"--stop", "3000"}, opts{stop: 3000}, false},
		{[]string{"--kill", "3000"}, opts{kill: 3000}, false},
		{[]string{"--stop", "3000", "--udp"}, opts{stop: 3000, showUDP: true}, false},
		{[]string{"--stop"}, opts{}, true},                     // missing port
		{[]string{"--kill", "notaport"}, opts{}, true},         // invalid port
		{[]string{"--stop", "1", "--kill", "2"}, opts{}, true}, // both
		{[]string{"99999"}, opts{}, true},                      // out of range
		{[]string{"bogus"}, opts{}, true},                      // not a flag
	}
	for _, tc := range cases {
		got, err := parseOpts(tc.args)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseOpts(%v) err = %v, wantErr %v", tc.args, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("parseOpts(%v) = %+v, want %+v", tc.args, got, tc.want)
		}
	}
}

func TestSignalPort(t *testing.T) {
	old := signalProcess
	var got []struct {
		pid int
		sig syscall.Signal
	}
	signalProcess = func(pid int, sig syscall.Signal) error {
		got = append(got, struct {
			pid int
			sig syscall.Signal
		}{pid, sig})
		return nil
	}
	defer func() { signalProcess = old }()

	rows := []Row{
		{Local: "0.0.0.0:8080", Process: "devserver", PID: 4242},
		{Local: "127.0.0.1:8080", Process: "devserver", PID: 4242},
		{Local: "0.0.0.0:8080", PID: 0}, // someone else's socket
		{Local: "0.0.0.0:9090", Process: "other", PID: 77},
	}
	var out strings.Builder
	if err := signalPort(&out, rows, 8080, syscall.SIGTERM, "SIGTERM"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].pid != 4242 || got[0].sig != syscall.SIGTERM {
		t.Errorf("signals = %+v, want exactly one SIGTERM to 4242", got)
	}
	if !strings.Contains(out.String(), "SIGTERM sent to 4242 (devserver)") {
		t.Errorf("output missing confirmation:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "another user") {
		t.Errorf("output missing unowned-socket notice:\n%s", out.String())
	}
}

func TestSignalPortErrors(t *testing.T) {
	old := signalProcess
	signalProcess = func(pid int, sig syscall.Signal) error { return nil }
	defer func() { signalProcess = old }()

	if err := signalPort(io.Discard, nil, 8080, syscall.SIGTERM, "SIGTERM"); err == nil ||
		!strings.Contains(err.Error(), "nothing listening") {
		t.Errorf("expect nothing-listening error, got %v", err)
	}
	unowned := []Row{{Local: "0.0.0.0:22"}}
	if err := signalPort(io.Discard, unowned, 22, syscall.SIGTERM, "SIGTERM"); err == nil ||
		!strings.Contains(err.Error(), "sudo") {
		t.Errorf("expect sudo hint for unowned socket, got %v", err)
	}
}
