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
	svc := make(map[int]string)
	parseServices(fixture(t, "services.txt"), svc)
	// Default view: TCP over IPv4, grouped by process.
	golden(t, "ports_render.golden", Render(&Report{Rows: procRows(t), Svc: svc}, false, false))
}

func TestRenderEmpty(t *testing.T) {
	if got := Render(&Report{}, false, false); got != "No listening ports found.\n" {
		t.Errorf("Render(empty) = %q", got)
	}
}

func TestRenderDeterministic(t *testing.T) {
	rep := &Report{Rows: procRows(t)}
	if a, b := Render(rep, false, true), Render(rep, false, true); a != b {
		t.Error("Render must be deterministic")
	}
}

func TestRenderGroupsByPID(t *testing.T) {
	got := Render(&Report{Rows: procRows(t)}, false, false)
	for _, want := range []string{"PROCESS", "chromium", "123", "127.0.0.1:46747", "0.0.0.0:8766"} {
		if !strings.Contains(got, want) {
			t.Errorf("grouped output missing %q:\n%s", want, got)
		}
	}
	// A continuation socket must not print its process/PID again.
	if strings.Count(got, "chromium") != 1 {
		t.Errorf("process should appear once per group:\n%s", got)
	}
}

func TestRenderUDPRequiresFlag(t *testing.T) {
	def := Render(&Report{Rows: procRows(t)}, false, false)
	if strings.Contains(def, "udp") {
		t.Error("UDP sockets should be hidden unless --udp")
	}
	// Without --udp everything shown is TCP, so PROTO is redundant.
	if strings.Contains(def, "PROTO") || strings.Contains(def, "tcp ") {
		t.Errorf("PROTO column should be dropped without --udp:\n%s", def)
	}
	withUDP := Render(&Report{Rows: procRows(t)}, false, true)
	if !strings.Contains(withUDP, "udp") {
		t.Error("--udp should include UDP sockets")
	}
	if !strings.Contains(withUDP, "PROTO") {
		t.Error("--udp should keep the PROTO column")
	}
}

func TestRenderIPv6RequiresFlag(t *testing.T) {
	def := Render(&Report{Rows: procRows(t)}, false, false)
	if strings.Contains(def, "[::") {
		t.Error("IPv6 sockets should be hidden unless --ipv6")
	}
	withV6 := Render(&Report{Rows: procRows(t)}, true, false)
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

func TestDecodeProcAddr(t *testing.T) {
	cases := map[string]string{
		"0100007F":                         "127.0.0.1",
		"00000000":                         "0.0.0.0",
		"9701A8C0":                         "192.168.1.151",
		"00000000000000000000000000000000": "[::]",
		"00000000000000000000000001000000": "[::1]",
		"5C117AFD0000E0A100000000398F361A": "[fd7a:115c:a1e0::1a36:8f39]",
	}
	for in, want := range cases {
		if got := decodeProcAddr(in); got != want {
			t.Errorf("decodeProcAddr(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseProcLocal(t *testing.T) {
	cases := map[string]struct {
		host string
		port int
	}{
		"0100007F:B69B":                         {"127.0.0.1", 46747},
		"00000000:0016":                         {"0.0.0.0", 22},
		"00000000000000000000000000000000:0277": {"[::]", 631},
		"5C117AFD0000E0A100000000398F361A:CCBC": {"[fd7a:115c:a1e0::1a36:8f39]", 52412},
	}
	for in, want := range cases {
		host, port, ok := parseProcLocal(in)
		if !ok || host != want.host || port != want.port {
			t.Errorf("parseProcLocal(%q) = %q %d %v, want %q %d", in, host, port, ok, want.host, want.port)
		}
	}
}

func TestParseProcNet(t *testing.T) {
	tcp, err := parseProcNet(fixture(t, "proc_tcp.txt"), "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(tcp) != 2 || tcp[0].row.Local != "127.0.0.1:46747" || tcp[0].inode != 8759300 ||
		tcp[1].row.Local != "0.0.0.0:8766" || tcp[1].inode != 2501496 {
		t.Errorf("tcp entries = %+v", tcp)
	}
	tcp6, err := parseProcNet(fixture(t, "proc_tcp6.txt"), "tcp")
	if err != nil {
		t.Fatal(err)
	}
	if len(tcp6) != 3 {
		t.Fatalf("tcp6 entries = %d, want 3 (one ESTABLISHED dropped)", len(tcp6))
	}
	if tcp6[0].row.Local != "[::]:22" || tcp6[1].row.Local != "[::1]:631" ||
		tcp6[2].row.Local != "[fd7a:115c:a1e0::1a36:8f39]:52412" {
		t.Errorf("tcp6 locals = %+v", tcp6)
	}
	udp, err := parseProcNet(fixture(t, "proc_udp.txt"), "udp")
	if err != nil {
		t.Fatal(err)
	}
	if len(udp) != 2 || udp[0].row.Local != "192.168.1.151:50818" || udp[0].inode != 13759695 ||
		udp[1].row.Local != "0.0.0.0:1900" || udp[1].inode != 40054 {
		t.Errorf("udp entries = %+v", udp)
	}
}

func TestPidsForInodes(t *testing.T) {
	root := fakeProcTree(t)
	unreadable := filepath.Join(root, "111")
	if err := os.MkdirAll(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadable, 0o755)

	owner := pidsForInodes(root, map[uint64]bool{8759300: true, 18552: true, 13759695: true, 999999: true})
	want := map[uint64]int{8759300: 123, 18552: 456, 13759695: 789}
	for inode, pid := range want {
		if owner[inode] != pid {
			t.Errorf("inode %d owner = %d, want %d", inode, owner[inode], pid)
		}
	}
	if _, ok := owner[999999]; ok {
		t.Error("unknown inode must not be claimed")
	}
}

func TestPortsFromProc(t *testing.T) {
	rows, err := portsFromProc(fakeProcTree(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7 (2 tcp, 3 tcp6, 2 udp)", len(rows))
	}
	byLocal := make(map[string]Row)
	for _, r := range rows {
		byLocal[r.Local] = r
	}
	got := byLocal["127.0.0.1:46747"]
	if got.PID != 123 || got.Process != "chromium" {
		t.Errorf("127.0.0.1:46747 = %+v, want pid 123 chromium", got)
	}
	if got := byLocal["0.0.0.0:8766"]; got.PID != 0 || got.Process != "" {
		t.Errorf("0.0.0.0:8766 should be unowned, got %+v", got)
	}
	if got := byLocal["[::]:22"]; got.PID != 456 || got.Process != "sshd" {
		t.Errorf("[::]:22 = %+v, want pid 456 sshd", got)
	}
	if got := byLocal["192.168.1.151:50818"]; got.PID != 789 || got.Process != "tailscaled" {
		t.Errorf("192.168.1.151:50818 = %+v, want pid 789 tailscaled", got)
	}
	if got := byLocal["0.0.0.0:1900"]; got.PID != 0 {
		t.Errorf("0.0.0.0:1900 should be unowned, got %+v", got)
	}
}

func TestRenderFromProc(t *testing.T) {
	svc := make(map[int]string)
	parseServices(fixture(t, "services.txt"), svc)
	rep := &Report{Rows: procRows(t), Svc: svc}
	golden(t, "ports_proc_render.golden", Render(rep, true, true))
	// The default view still hides UDP and IPv6 sockets.
	def := Render(rep, false, false)
	if strings.Contains(def, "udp") || strings.Contains(def, "[::") {
		t.Errorf("default view must hide udp/ipv6:\n%s", def)
	}
}

func TestSample(t *testing.T) {
	oldRoot, oldServices := procRoot, servicesPath
	procRoot = fakeProcTree(t)
	data, err := os.ReadFile(filepath.Join("testdata", "services.txt"))
	if err != nil {
		t.Fatal(err)
	}
	svcFile := filepath.Join(t.TempDir(), "services")
	if err := os.WriteFile(svcFile, data, 0o644); err != nil {
		t.Fatal(err)
	}
	servicesPath = svcFile
	defer func() { procRoot, servicesPath = oldRoot, oldServices }()

	rep, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Rows) == 0 {
		t.Fatal("Sample should read /proc sockets")
	}
	for _, r := range rep.Rows {
		if r.Local == "127.0.0.1:46747" && (r.PID != 123 || r.Process != "chromium") {
			t.Errorf("Sample should resolve pids, got %+v", r)
		}
	}
	if rep.Svc[22] != "ssh" || rep.Svc[631] != "ipp" {
		t.Errorf("Sample should attach service names, got %v", rep.Svc)
	}
}

func TestSampleMissingServices(t *testing.T) {
	oldRoot, oldServices := procRoot, servicesPath
	procRoot = fakeProcTree(t)
	servicesPath = filepath.Join(t.TempDir(), "no-services")
	defer func() { procRoot, servicesPath = oldRoot, oldServices }()

	rep, err := Sample()
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Rows) == 0 {
		t.Fatal("Sample should read /proc sockets without /etc/services")
	}
}

// procRows reads the fake /proc tree once for render tests.
func procRows(t *testing.T) []Row {
	t.Helper()
	rows, err := portsFromProc(fakeProcTree(t))
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// fakeProcTree builds a fake /proc with crafted /proc/net files and a couple
// of processes owning socket fds, mirroring the on-disk layout portsFromProc
// reads.
func fakeProcTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, fam := range []string{"tcp", "tcp6", "udp"} {
		data, err := os.ReadFile(filepath.Join("testdata", "proc_"+fam+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		write("net/"+fam, string(data))
	}
	write("123/comm", "chromium\n")
	write("456/comm", "sshd\n")
	write("789/comm", "tailscaled\n")
	symlink := func(pid, fd, target string) {
		t.Helper()
		p := filepath.Join(root, pid, "fd", fd)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, p); err != nil {
			t.Fatal(err)
		}
	}
	symlink("123", "5", "socket:[8759300]")
	symlink("123", "6", "pipe:[999]") // non-socket fds are ignored
	symlink("456", "7", "socket:[18552]")
	symlink("789", "9", "socket:[13759695]")
	return root
}
