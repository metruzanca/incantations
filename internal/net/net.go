// Package net reports per-interface IP addresses and live transfer rates, by
// sampling /proc/net/dev over a short window.
package net

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
	"github.com/metruzanca/incantations/internal/units"
)

// Counter is one interface's byte and error counters from /proc/net/dev.
type Counter struct {
	RXBytes, RXErrs, RXDrop uint64
	TXBytes, TXErrs, TXDrop uint64
}

// NetDev maps interface names to their raw counters.
type NetDev map[string]Counter

// Iface is one interface with its resolved addresses and live rates.
type Iface struct {
	Name  string
	Addrs []string
	Up    bool
	RX    uint64 // bytes per second
	TX    uint64 // bytes per second
}

// Report bundles the interfaces captured in a sampling window.
type Report struct {
	Ifaces []Iface
}

// parseDev parses /proc/net/dev contents (a header block followed by one
// "iface: bytes packets errs …" line per interface).
func parseDev(r io.Reader) (NetDev, error) {
	dev := make(NetDev)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		i := strings.IndexByte(line, ':')
		if i < 0 {
			continue
		}
		name := strings.TrimSpace(line[:i])
		fields := strings.Fields(line[i+1:])
		if len(fields) < 12 {
			continue
		}
		dev[name] = Counter{
			RXBytes: atoi(fields[0]),
			RXErrs:  atoi(fields[2]),
			RXDrop:  atoi(fields[3]),
			TXBytes: atoi(fields[8]),
			TXErrs:  atoi(fields[10]),
			TXDrop:  atoi(fields[11]),
		}
	}
	return dev, sc.Err()
}

func atoi(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// computeRates turns two counter snapshots into bytes-per-second rates for
// every interface present in both. A counter that went backwards (reset) is
// treated as zero for the window.
func computeRates(before, after NetDev, interval time.Duration) NetDev {
	rates := make(NetDev)
	secs := interval.Seconds()
	for name, b := range before {
		a, ok := after[name]
		if !ok {
			continue
		}
		rates[name] = Counter{
			RXBytes: rate(b.RXBytes, a.RXBytes, secs),
			TXBytes: rate(b.TXBytes, a.TXBytes, secs),
		}
	}
	return rates
}

func rate(before, after uint64, secs float64) uint64 {
	if after <= before {
		return 0
	}
	return uint64(float64(after-before) / secs)
}

// Spec registers the net command. Sampling takes about a second so rates are
// meaningful; -a also shows the loopback interface (hidden by default), and
// --ipv6 shows IPv6 addresses (hidden by default).
func Spec() command.Entry {
	return command.Entry{
		Name:    "net",
		Summary: "show network addresses and live transfer rates (takes ~1s)",
		Help: `Usage:
  incantations net [-a|--all] [--ipv6]

Shows each active network interface, its IP addresses, and its current
download (RX) and upload (TX) speed. Rates are measured over about a second,
so this command takes roughly that long.

The loopback interface and IPv6 addresses are noise for most people, so they
are hidden by default: pass -a or --all to show loopback, and --ipv6 to show
IPv6 addresses.`,
		Run: func(args []string, stdout io.Writer) error {
			all := false
			showV6 := false
			for _, a := range args {
				switch a {
				case "-a", "--all":
					all = true
				case "--ipv6":
					showV6 = true
				default:
					return fmt.Errorf("usage: incantations net [-a|--all] [--ipv6]")
				}
			}
			rep, err := Sample(time.Second)
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, Render(rep, all, showV6))
			return err
		},
	}
}

// Render formats the report as a table sorted by interface name. Only
// interfaces that are up are listed; loopback and IPv6 addresses are hidden
// unless all and showV6 are set. An interface that has addresses only of the
// hidden kind still appears (so you can see it's there), showing "-".
func Render(r *Report, all, showV6 bool) string {
	var ifaces []Iface
	for _, i := range r.Ifaces {
		if i.Name == "lo" && !all {
			continue
		}
		if !i.Up && i.RX == 0 && i.TX == 0 {
			continue
		}
		ifaces = append(ifaces, i)
	}
	if len(ifaces) == 0 {
		return "No active network interfaces.\n"
	}
	sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })
	rows := make([][]string, 0, len(ifaces))
	for _, i := range ifaces {
		addr := "-"
		if addrs := visibleAddrs(i.Addrs, showV6); len(addrs) > 0 {
			addr = strings.Join(addrs, " ")
		}
		rx, tx := units.HumanRate(i.RX), units.HumanRate(i.TX)
		if i.RX == 0 {
			rx = "-"
		}
		if i.TX == 0 {
			tx = "-"
		}
		rows = append(rows, []string{i.Name, "up", addr, rx, tx})
	}
	return ui.NewTable(
		[]string{"INTERFACE", "STATUS", "IP ADDRESSES", "RECEIVE", "TRANSMIT"},
		[]bool{false, false, false, true, true},
		rows,
	) + "\n"
}

// visibleAddrs returns the addresses to show: IPv4 always, IPv6 only when
// showV6 is set.
func visibleAddrs(addrs []string, showV6 bool) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if showV6 || net.ParseIP(a).To4() != nil {
			out = append(out, a)
		}
	}
	return out
}
