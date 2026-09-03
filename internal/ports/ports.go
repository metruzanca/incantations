// Package ports reports which TCP and UDP ports are listening and which
// process owns each, by parsing `ss -ltulpn` output.
package ports

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
)

// Row is a single listening socket.
type Row struct {
	Proto   string // tcp, tcp6, udp, udp6
	Local   string // address:port, e.g. 0.0.0.0:22 or [::]:631
	Process string // empty when the process is not ours (needs root)
	PID     int
}

// Report bundles the parsed sockets and any service-name lookups.
type Report struct {
	Rows []Row
	Svc  map[int]string // port -> service name from /etc/services
}

// netids are the only socket types `ss -ltu` can emit; anything else is a
// wrapped continuation line to skip.
var netids = map[string]bool{"tcp": true, "tcp6": true, "udp": true, "udp6": true}

// parseSs parses `ss -ltulpn` output into rows. Socket addresses never contain
// spaces, so the process field (users:(...)) is whatever follows column 6.
func parseSs(r io.Reader) ([]Row, error) {
	var rows []Row
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 || !netids[fields[0]] {
			continue // header, blank, or a wrapped continuation line
		}
		process, pid := parseUsers(fields[6:])
		rows = append(rows, Row{
			Proto:   fields[0],
			Local:   fields[4],
			Process: process,
			PID:     pid,
		})
	}
	return rows, sc.Err()
}

// parseUsers extracts the first process name and PID from the ss process
// column, which looks like users:(("sshd",pid=1234,fd=5)).
func parseUsers(fields []string) (string, int) {
	joined := strings.Join(fields, " ")
	open := strings.Index(joined, "((")
	if open < 0 || open+2 >= len(joined) || joined[open+2] != '"' {
		return "", 0
	}
	start := open + 3
	var name string
	if end := strings.IndexByte(joined[start:], '"'); end >= 0 {
		name = joined[start : start+end]
	}
	pid := 0
	if i := strings.Index(joined, "pid="); i >= 0 {
		pid, _ = strconv.Atoi(portDigits(joined[i+4:]))
	}
	return name, pid
}

// portDigits scans digits off the front of s (the PID run).
func portDigits(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// parseServices reads /etc/services-style contents ("http 80/tcp") into a
// port-to-name map. First name wins.
func parseServices(r io.Reader, out map[int]string) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[1], "#") {
			continue
		}
		proto := fields[1]
		if i := strings.IndexByte(proto, '/'); i >= 0 {
			proto = proto[:i]
		}
		port, err := strconv.Atoi(proto)
		if err != nil || port <= 0 {
			continue
		}
		if _, ok := out[port]; !ok {
			out[port] = fields[0]
		}
	}
}

// localPort extracts the numeric port from an ss local address like
// "0.0.0.0:22", "[::]:631", or "*:60731".
func localPort(local string) int {
	if i := strings.LastIndexByte(local, ':'); i >= 0 && i+1 < len(local) {
		if p, err := strconv.Atoi(local[i+1:]); err == nil {
			return p
		}
	}
	return 0
}

// Spec registers the ports command. An optional numeric PORT filters the
// listing to sockets on that port; --udp and --ipv6 include sockets that are
// hidden by default; --stop and --kill signal the process on a port.
func Spec() command.Entry {
	return command.Entry{
		Name:    "ports",
		Summary: "show which TCP ports are listening and what uses each; --stop/--kill a port",
		Help: `Usage:
  incantations ports [PORT] [--udp] [--ipv6]
  incantations ports --stop PORT
  incantations ports --kill PORT

Shows which TCP ports are listening and what's using each one, grouped by
process. Data comes from ss -ltulpn, the modern replacement for netstat.

UDP sockets and IPv6 addresses are usually not what you're looking for, so
they are hidden by default: pass --udp to include UDP and --ipv6 to include
IPv6. Pass a port number (e.g. ports 8080) to check one port.

--stop and --kill send SIGTERM or SIGKILL to whatever is listening on that
port, so a stray dev server can be stopped without hunting down its pid
first. Process names and PIDs for other users' processes need root; without
it those sockets are grouped under "-" and cannot be signaled.
`,
		Run: func(args []string, stdout io.Writer) error {
			o, err := parseOpts(args)
			if err != nil {
				return err
			}
			rep, err := Sample()
			if err != nil {
				return err
			}
			if o.stop > 0 || o.kill > 0 {
				sig, sigName, port := termSignal(), "SIGTERM", o.stop
				if o.kill > 0 {
					sig, sigName, port = killSignal(), "SIGKILL", o.kill
				}
				return signalPort(stdout, rep.Rows, port, sig, sigName)
			}
			if o.filter > 0 {
				filtered := &Report{Svc: rep.Svc}
				for _, row := range rep.Rows {
					if localPort(row.Local) == o.filter {
						filtered.Rows = append(filtered.Rows, row)
					}
				}
				rep = filtered
			}
			_, err = io.WriteString(stdout, Render(rep, o.showV6, o.showUDP))
			return err
		},
	}
}

// opts carries the parsed command-line flags for one invocation.
type opts struct {
	filter  int
	stop    int
	kill    int
	showV6  bool
	showUDP bool
}

// parseOpts parses flags and a bare PORT number. --stop and --kill take a
// port argument and are mutually exclusive.
func parseOpts(args []string) (opts, error) {
	var o opts
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--udp":
			o.showUDP = true
		case "--ipv6":
			o.showV6 = true
		case "--stop", "--kill":
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s needs a port (e.g. %s 8080)", args[i], args[i])
			}
			p, err := parsePort(args[i+1])
			if err != nil {
				return o, fmt.Errorf("%s: %w", args[i], err)
			}
			if args[i] == "--stop" {
				o.stop = p
			} else {
				o.kill = p
			}
			i++
		default:
			p, err := parsePort(args[i])
			if err != nil {
				return o, fmt.Errorf("usage: incantations ports [PORT] [--udp] [--ipv6] [--stop PORT] [--kill PORT]")
			}
			o.filter = p
		}
	}
	if o.stop > 0 && o.kill > 0 {
		return o, fmt.Errorf("cannot use --stop and --kill together")
	}
	return o, nil
}

// parsePort validates a port argument.
func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return p, nil
}

// signalPort sends sig to every owned process listening on the given port and
// prints what it did. Sockets whose owner we can't see (other users, so no
// PID) are reported rather than silently skipped.
func signalPort(w io.Writer, rows []Row, port int, sig syscall.Signal, sigName string) error {
	byPid := make(map[int]string)
	unowned := false
	for _, row := range rows {
		if localPort(row.Local) != port {
			continue
		}
		if row.PID > 0 {
			if _, ok := byPid[row.PID]; !ok {
				byPid[row.PID] = row.Process
			}
		} else {
			unowned = true
		}
	}
	if len(byPid) == 0 {
		if unowned {
			return fmt.Errorf("port %d is owned by a process you can't see; run with sudo so its pid shows up", port)
		}
		return fmt.Errorf("nothing listening on port %d", port)
	}
	pids := make([]int, 0, len(byPid))
	for pid := range byPid {
		pids = append(pids, pid)
	}
	sort.Ints(pids)
	var errs []string
	for _, pid := range pids {
		name := byPid[pid]
		if name == "" {
			name = "-"
		}
		if err := signalProcess(pid, sig); err != nil {
			errs = append(errs, fmt.Sprintf("%d: %v", pid, err))
			continue
		}
		fmt.Fprintf(w, "%s sent to %d (%s)\n", sigName, pid, name)
	}
	if len(errs) > 0 {
		return fmt.Errorf("could not %s all listeners: %s", sigName, strings.Join(errs, "; "))
	}
	if unowned {
		fmt.Fprintln(w, "some sockets on that port belong to another user and were not signaled")
	}
	return nil
}

// group is a process holding one or more listening sockets, identified by PID.
type group struct {
	pid   int
	name  string
	ports []Row
}

// groupByPID lumps sockets by owning process. Sockets with no PID (owned by
// other users, or kernel listeners) form a trailing "-" group.
func groupByPID(rows []Row) []*group {
	byPID := make(map[int]*group, len(rows))
	groups := make([]*group, 0, len(rows))
	for _, row := range rows {
		g := byPID[row.PID]
		if g == nil {
			g = &group{pid: row.PID, name: row.Process}
			byPID[row.PID] = g
			groups = append(groups, g)
		}
		g.ports = append(g.ports, row)
	}
	sort.Slice(groups, func(i, j int) bool {
		a, b := groups[i].pid, groups[j].pid
		if a == 0 {
			return false // unowned sockets listed last
		}
		if b == 0 {
			return true
		}
		return a < b
	})
	for _, g := range groups {
		sort.Slice(g.ports, func(i, j int) bool {
			a, b := localPort(g.ports[i].Local), localPort(g.ports[j].Local)
			if a != b {
				return a < b
			}
			return g.ports[i].Proto < g.ports[j].Proto
		})
	}
	return groups
}

// Render formats the report as one table grouped by owning process: each
// process's name and PID appear on its first row only, with its sockets under
// it. The PROTO column is redundant when only TCP is shown, so it is dropped
// unless --udp brought UDP sockets in. IPv6 sockets are hidden unless showV6,
// since TCP over IPv4 is what people actually look for. Well-known ports are
// labeled with their /etc/services name when known.
func Render(r *Report, showV6, showUDP bool) string {
	rows := make([]Row, 0, len(r.Rows))
	for _, row := range r.Rows {
		if !showUDP && strings.HasPrefix(row.Proto, "udp") {
			continue
		}
		// IPv6 sockets are bound to a bracketed address like [::]:8080; some
		// ss versions also label the netid tcp6/udp6.
		if !showV6 && (strings.HasSuffix(row.Proto, "6") || strings.HasPrefix(row.Local, "[")) {
			continue
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return "No listening ports found.\n"
	}
	groups := groupByPID(rows)
	headers := []string{"PROCESS", "PID", "LOCAL ADDRESS"}
	right := []bool{false, true, false}
	if showUDP {
		headers = []string{"PROCESS", "PID", "PROTO", "LOCAL ADDRESS"}
		right = []bool{false, true, false, false}
	}
	out := make([][]string, 0, len(rows))
	for _, g := range groups {
		name, pid := "-", "-"
		if g.pid > 0 {
			name, pid = g.name, strconv.Itoa(g.pid)
		}
		for i, p := range g.ports {
			local := p.Local
			if svc, ok := r.Svc[localPort(local)]; ok {
				local = fmt.Sprintf("%s (%s)", local, svc)
			}
			var row []string
			if showUDP {
				row = []string{"", "", p.Proto, local}
			} else {
				row = []string{"", "", local}
			}
			if i == 0 {
				row[0], row[1] = name, pid
			}
			out = append(out, row)
		}
	}
	return ui.NewTable(headers, right, out) + "\n"
}
