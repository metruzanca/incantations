// Package ui renders aligned tables using charmbracelet. Output is plain
// (deterministic) unless Styled is enabled, which happens only when stdout is
// a real terminal so piped output stays clean.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// Styled enables color styling. main.go sets it when stdout is a terminal;
// tests and pipelines render plain.
var Styled = false

// NewTable renders a table with the given headers and rows. right marks the
// numeric columns that should be right-justified. Column widths are sized to
// the widest header or cell.
func NewTable(headers []string, right []bool, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if l := len(c); l > widths[i] {
				widths[i] = l
			}
		}
	}
	cols := make([]table.Column, len(headers))
	for i := range headers {
		title := headers[i]
		if right[i] && len(title) < widths[i] {
			title = fmt.Sprintf("%*s", widths[i], title)
		}
		cols[i] = table.Column{Title: title, Width: widths[i]}
	}
	trs := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		row := make(table.Row, len(headers))
		for i, c := range r {
			if right[i] && len(c) < widths[i] {
				c = fmt.Sprintf("%*s", widths[i], c)
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
			Header:   header,
			Cell:     cell,
			Selected: cell, // bubbletea always selects row 0; render it like the rest
		}),
	)
	return strings.TrimSuffix(t.View(), "\n")
}
