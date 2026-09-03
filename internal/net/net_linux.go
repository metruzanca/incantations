//go:build linux

package net

import (
	"net"
	"os"
	"time"
)

// devPath is a variable so tests can point at a fixture.
var devPath = "/proc/net/dev"

// Sample reads interface counters twice, interval apart, and resolves each
// interface's IP addresses and link state. The sleep between the two reads is
// what makes the rates meaningful.
func Sample(interval time.Duration) (*Report, error) {
	before, err := readDev()
	if err != nil {
		return nil, err
	}
	time.Sleep(interval)
	after, err := readDev()
	if err != nil {
		return nil, err
	}
	rates := computeRates(before, after, interval)

	ifaces := make([]Iface, 0, len(rates))
	for name := range rates {
		i := Iface{Name: name, RX: rates[name].RXBytes, TX: rates[name].TXBytes}
		if ni, err := net.InterfaceByName(name); err == nil {
			i.Up = ni.Flags&net.FlagUp != 0
		}
		i.Addrs = ifaceAddrs(name)
		ifaces = append(ifaces, i)
	}
	return &Report{Ifaces: ifaces}, nil
}

func readDev() (NetDev, error) {
	f, err := os.Open(devPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseDev(f)
}

// ifaceAddrs resolves an interface's IP addresses via the standard library so
// the addresses stay correct without parsing iproute2 output.
func ifaceAddrs(name string) []string {
	ni, err := net.InterfaceByName(name)
	if err != nil {
		return nil
	}
	addrs, err := ni.Addrs()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			out = append(out, ipnet.IP.String())
		}
	}
	return out
}
