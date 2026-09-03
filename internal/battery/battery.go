// Package battery reports charge state, drain/charge rate, and time remaining
// for a laptop battery by reading sysfs power-supply data.
package battery

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"github.com/metruzanca/incantations/internal/ui"
	"github.com/metruzanca/incantations/internal/units"
)

// Battery holds the figures that back the report, all expressed in
// micro-watt-hours (energy) and micro-watts (power), which is what sysfs
// exposes. Charge in micro-amp-hours is converted using the current voltage.
type Battery struct {
	Status       string // Charging, Discharging, Full, or Not charging
	Percent      uint64 // 0-100
	EnergyNow    uint64 // µWh
	EnergyFull   uint64 // µWh
	EnergyDesign uint64 // µWh
	PowerNow     uint64 // µW
}

// Report bundles the full result of a capture. Found is false when the system
// has no battery at all (most desktops), which is not an error.
type Report struct {
	Found bool
	B     Battery
}

// parseUevent parses a power-supply uevent file (KEY=VALUE lines). Batteries
// expose either ENERGY_* (µWh) or CHARGE_* (µAh) plus VOLTAGE_NOW (µV), so
// CHARGE* is converted with the current voltage. Likewise, power is taken
// from POWER_NOW when present, otherwise derived from CURRENT_NOW × voltage.
func parseUevent(r io.Reader) (Battery, error) {
	var b Battery
	vals := make(map[string]string)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if i := strings.IndexByte(sc.Text(), '='); i > 0 {
			vals[sc.Text()[:i]] = sc.Text()[i+1:]
		}
	}
	if err := sc.Err(); err != nil {
		return b, err
	}
	b.Status = vals["POWER_SUPPLY_STATUS"]
	if v, err := strconv.ParseUint(vals["POWER_SUPPLY_CAPACITY"], 10, 64); err == nil {
		b.Percent = v
	}
	voltage := parseUint(vals["POWER_SUPPLY_VOLTAGE_NOW"])
	b.EnergyNow = energy(vals, "POWER_SUPPLY_ENERGY_NOW", "POWER_SUPPLY_CHARGE_NOW", voltage)
	b.EnergyFull = energy(vals, "POWER_SUPPLY_ENERGY_FULL", "POWER_SUPPLY_CHARGE_FULL", voltage)
	b.EnergyDesign = energy(vals, "POWER_SUPPLY_ENERGY_FULL_DESIGN", "POWER_SUPPLY_CHARGE_FULL_DESIGN", voltage)
	if v := parseUint(vals["POWER_SUPPLY_POWER_NOW"]); v > 0 {
		b.PowerNow = v
	} else if current := parseUint(vals["POWER_SUPPLY_CURRENT_NOW"]); current > 0 {
		b.PowerNow = current * voltage / 1e6
	}
	return b, nil
}

// energy reads a micro-watt-hour field, falling back to the micro-amp-hour
// field scaled by the voltage (µV). A zero voltage leaves the fallback at 0.
func energy(vals map[string]string, uwhKey, uahKey string, voltage uint64) uint64 {
	if v := parseUint(vals[uwhKey]); v > 0 {
		return v
	}
	if voltage == 0 {
		return 0
	}
	return parseUint(vals[uahKey]) * voltage / 1e6
}

func parseUint(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// Spec registers the battery command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "battery",
		Summary: "show battery charge, drain rate, and time remaining",
		Help: `Usage:
  incantations battery

Shows charge state, a usage bar, current drain/charge rate, and estimated time
remaining (to empty when discharging, to full when charging) for a laptop
battery. Print "No battery found." on machines without one.`,
		Run: func(args []string, stdout io.Writer) error {
			rep, err := Sample()
			if err != nil {
				return err
			}
			_, err = io.WriteString(stdout, Render(rep))
			return err
		},
	}
}

// Render formats a report for humans. A missing battery becomes a single
// friendly line rather than an error, so the shell utility never looks broken
// on desktops.
func Render(r *Report) string {
	if !r.Found {
		return "No battery found.\n"
	}
	b := r.B
	rows := [][]string{{
		"BATTERY",
		b.Status,
		usageCell(b),
	}}
	var out strings.Builder
	out.WriteString(ui.NewTable(
		[]string{"TYPE", "STATE", "USAGE"},
		[]bool{false, false, false},
		rows,
	))
	out.WriteString("\n")

	var meta []string
	if b.PowerNow > 0 {
		meta = append(meta, "Rate "+units.HumanPower(b.PowerNow))
	}
	if d := remaining(b); d > 0 {
		switch b.Status {
		case "Discharging":
			meta = append(meta, units.HumanDuration(d)+" left")
		case "Charging":
			meta = append(meta, units.HumanDuration(d)+" to full")
		}
	}
	if b.EnergyDesign > 0 && b.EnergyFull > 0 && b.EnergyFull <= b.EnergyDesign {
		meta = append(meta, fmt.Sprintf("Design health %.0f%%", units.Pct(b.EnergyFull, b.EnergyDesign)))
	}
	if len(meta) > 0 {
		out.WriteString(strings.Join(meta, " · ") + "\n")
	}
	return out.String()
}

// remaining returns the seconds to empty (discharging) or to full (charging)
// given the current power draw. Zero when there is nothing to compute.
func remaining(b Battery) int64 {
	if b.PowerNow == 0 {
		return 0
	}
	hours := float64(b.EnergyNow) / float64(b.PowerNow)
	switch b.Status {
	case "Discharging":
		return int64(hours * 3600)
	case "Charging":
		if b.EnergyFull > b.EnergyNow {
			return int64(float64(b.EnergyFull-b.EnergyNow) / float64(b.PowerNow) * 3600)
		}
	}
	return 0
}

// usageCell renders a progress bar, the charge percentage, and the
// "now/full (free)" summary for the battery.
func usageCell(b Battery) string {
	return fmt.Sprintf("%s %3.0f%%  %s/%s (%s left)",
		ui.Bar(float64(b.Percent)/100, 20), float64(b.Percent),
		units.CompactEnergy(b.EnergyNow), units.CompactEnergy(b.EnergyFull),
		units.CompactEnergy(b.EnergyFull-b.EnergyNow))
}
