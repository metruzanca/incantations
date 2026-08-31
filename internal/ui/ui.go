// Package ui renders aligned tables using charmbracelet. Output is plain
// (deterministic) unless Styled is enabled, which happens only when stdout is
// a real terminal so piped output stays clean.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Styled enables color styling. main.go sets it when stdout is a terminal;
// tests and pipelines render plain.
var Styled = false

// NewTable renders a table with the given headers and rows. right marks the
// numeric columns that should be right-justified. Column widths are sized to
// the widest header or cell, measured in terminal cells (multibyte block
// characters such as progress bars count as one cell).
func NewTable(headers []string, right []bool, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = ansi.StringWidth(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if l := ansi.StringWidth(c); l > widths[i] {
				widths[i] = l
			}
		}
	}
	padRight := func(s string, w int) string {
		return strings.Repeat(" ", w-ansi.StringWidth(s)) + s
	}
	cols := make([]table.Column, len(headers))
	for i := range headers {
		title := headers[i]
		if right[i] {
			title = padRight(title, widths[i])
		}
		cols[i] = table.Column{Title: title, Width: widths[i]}
	}
	trs := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		row := make(table.Row, len(headers))
		for i, c := range r {
			if right[i] {
				c = padRight(c, widths[i])
			}
			row[i] = c
		}
		trs = append(trs, row)
	}
	cell := lipgloss.NewStyle().Padding(0, 1)
	header := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if Styled {
		header = header.Foreground(lipgloss.Color("99"))
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(trs),
		table.WithHeight(len(rows)+1), // render exactly the data rows, no filler
		table.WithStyles(table.Styles{
			Header: header,
			Cell:   cell,
			// bubbletea always "selects" row 0 and re-renders it through
			// Selected as a whole. An empty style keeps it identical to the
			// other rows; padding here would shove the first row right.
			Selected: lipgloss.NewStyle(),
		}),
	)
	return strings.TrimSuffix(t.View(), "\n")
}

// ProgressBar renders a filled progress bar of the given cell width for a
// ratio in [0,1]. The percentage readout is left to the caller.
func ProgressBar(ratio float64, width int) string {
	m := progress.New(progress.WithWidth(width))
	m.ShowPercentage = false
	if Styled {
		m.FullColor = "#7D56F4"
		m.EmptyColor = "#3a3a3a"
	} else {
		m.FullColor = ""
		m.EmptyColor = ""
	}
	return m.ViewAs(ratio)
}
