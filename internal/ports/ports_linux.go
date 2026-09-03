//go:build linux

package ports

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// servicesPath is a variable so tests can point at a fixture.
var servicesPath = "/etc/services"

// procRoot is where /proc lives; a variable so tests can build a fake tree.
var procRoot = "/proc"

// signalProcess sends a signal to a pid. It is a variable so tests can record
// signals without touching real processes.
var signalProcess = func(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func termSignal() syscall.Signal { return syscall.SIGTERM }
func killSignal() syscall.Signal { return syscall.SIGKILL }

// Sample reads listening sockets from /proc/net and resolves their owners,
// then attaches service names from /etc/services (optional; a missing file is
// not fatal). Everything comes from /proc and sysfs files available on every
// Linux system — no external binaries.
func Sample() (*Report, error) {
	rows, err := portsFromProc(procRoot)
	if err != nil {
		return nil, err
	}
	svc := make(map[int]string)
	f, err := os.Open(servicesPath)
	if err == nil {
		parseServices(f, svc)
		f.Close()
	}
	return &Report{Rows: rows, Svc: svc}, nil
}

// portsFromProc reads /proc/net/{tcp,tcp6,udp,udp6}, keeps the listening
// sockets, and names the owning process for every one whose /proc tree is
// readable. Other users' /proc/<pid> trees are unreadable without root, so
// their sockets stay PID 0 and group under "-".
func portsFromProc(root string) ([]Row, error) {
	var entries []procEntry
	// The netid reads tcp/udp for both address families; the [ address prefix
	// is what marks a socket as ipv6 in Render.
	files := []struct{ file, proto string }{
		{"tcp", "tcp"}, {"tcp6", "tcp"}, {"udp", "udp"}, {"udp6", "udp"},
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(root, "net", f.file))
		if os.IsNotExist(err) {
			continue // a family with no sockets
		}
		if err != nil {
			return nil, err
		}
		es, err := parseProcNet(bytes.NewReader(data), f.proto)
		if err != nil {
			return nil, err
		}
		entries = append(entries, es...)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	want := make(map[uint64]bool, len(entries))
	for _, e := range entries {
		want[e.inode] = true
	}
	owner := pidsForInodes(root, want)
	rows := make([]Row, 0, len(entries))
	for _, e := range entries {
		row := e.row
		if pid, ok := owner[e.inode]; ok {
			row.PID = pid
			row.Process = procComm(root, pid)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// procEntry is a parsed socket along with the inode used to match it to a
// process.
type procEntry struct {
	row   Row
	inode uint64
}

// parseProcNet parses one /proc/net/{tcp,tcp6,udp,udp6} file. Mirroring ss -l,
// only listening TCP (state LISTEN) and bound UDP (state UNCONN) sockets are
// kept; rows are "local_addr st ... inode" all in hex.
func parseProcNet(r io.Reader, proto string) ([]procEntry, error) {
	var entries []procEntry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 || fields[0] == "sl" {
			continue // blank or header
		}
		state, err := strconv.ParseUint(fields[3], 16, 8)
		if err != nil {
			continue
		}
		listening := byte(state) == 0x0A // LISTEN
		bound := byte(state) == 0x07     // UNCONN
		if strings.HasPrefix(proto, "tcp") && !listening {
			continue
		}
		if strings.HasPrefix(proto, "udp") && !bound {
			continue
		}
		host, port, ok := parseProcLocal(fields[1])
		if !ok {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			continue
		}
		entries = append(entries, procEntry{
			row:   Row{Proto: proto, Local: fmt.Sprintf("%s:%d", host, port)},
			inode: inode,
		})
	}
	return entries, sc.Err()
}

// parseProcLocal decodes a /proc/net local address ("0100007F:B69B" or a 32-char
// IPv6 host) into a display host and the decimal port.
func parseProcLocal(local string) (string, int, bool) {
	i := strings.IndexByte(local, ':')
	if i < 0 {
		return "", 0, false
	}
	host := decodeProcAddr(local[:i])
	port, err := strconv.ParseUint(local[i+1:], 16, 16)
	if err != nil {
		return "", 0, false
	}
	return host, int(port), true
}

// decodeProcAddr converts the hex host field of a /proc/net row into the
// display form. IPv4 addresses are printed little-endian, so their bytes are
// reversed. IPv6 is printed as four 32-bit words in host byte order, so each
// word is stored little-endian into the address.
func decodeProcAddr(host string) string {
	if len(host) == 8 {
		return fmt.Sprintf("%d.%d.%d.%d",
			procByte(host, 6), procByte(host, 4), procByte(host, 2), procByte(host, 0))
	}
	if len(host) != 32 {
		return "?"
	}
	ip := make(net.IP, net.IPv6len)
	for w := 0; w < 4; w++ {
		word, _ := strconv.ParseUint(host[w*8:w*8+8], 16, 32)
		binary.LittleEndian.PutUint32(ip[w*4:w*4+4], uint32(word))
	}
	return "[" + ip.String() + "]"
}

func procByte(s string, start int) int {
	v, _ := strconv.ParseUint(s[start:start+2], 16, 8)
	return int(v)
}

// pidsForInodes finds the process owning each wanted socket inode by scanning
// /proc/<pid>/fd symlinks for their socket:[inode] targets. Unreadable pid
// directories (other users, kernel threads) are skipped.
func pidsForInodes(root string, want map[uint64]bool) map[uint64]int {
	owner := make(map[uint64]int, len(want))
	entries, err := os.ReadDir(root)
	if err != nil {
		return owner
	}
	for _, e := range entries {
		if len(owner) == len(want) {
			break // everything found; stop scanning fds
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || !e.IsDir() {
			continue
		}
		fdDir := filepath.Join(root, e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // other users' pids are not readable
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if inode, ok := fdSocketInode(target); ok && want[inode] {
				owner[inode] = pid
			}
		}
	}
	return owner
}

// fdSocketInode extracts the inode from a readlink target like
// "socket:[8759300]".
func fdSocketInode(target string) (uint64, bool) {
	if !strings.HasPrefix(target, "socket:[") {
		return 0, false
	}
	end := strings.LastIndexByte(target, ']')
	if end < 0 {
		return 0, false
	}
	inode, err := strconv.ParseUint(target[len("socket:["):end], 10, 64)
	if err != nil {
		return 0, false
	}
	return inode, true
}

// procComm returns the process name the kernel keeps in /proc/<pid>/comm (the
// same 15-character comm a listener shows).
func procComm(root string, pid int) string {
	data, err := os.ReadFile(filepath.Join(root, strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
