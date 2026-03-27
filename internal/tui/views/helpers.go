package views

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	colorTableHeader = tcell.NewRGBColor(56, 132, 244)
	colorSeparator   = tcell.NewRGBColor(50, 50, 65)
)

func rowMatchesFilter(filter string, fields ...string) bool {
	if filter == "" {
		return true
	}
	lf := strings.ToLower(filter)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), lf) {
			return true
		}
	}
	return false
}

// setTableHeader writes a bold blue header row (row 0).
// Data rows should start at row 1. Use SetFixed(1, 0).
func setTableHeader(table *tview.Table, columns ...string) {
	for i, col := range columns {
		cell := tview.NewTableCell(" " + col).
			SetTextColor(colorTableHeader).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, i, cell)
	}
}

func resultColor(result string) tcell.Color {
	switch result {
	case "SUCCESS":
		return tcell.NewRGBColor(72, 199, 142)
	case "FAILURE", "ERROR", "NODE_FAILURE":
		return tcell.NewRGBColor(235, 87, 87)
	case "RETRY_LIMIT", "LOST", "ABORTED", "TIMED_OUT", "DISK_FULL":
		return tcell.NewRGBColor(242, 201, 76)
	case "SKIPPED":
		return tcell.NewRGBColor(120, 120, 140)
	default:
		return tcell.NewRGBColor(56, 132, 244)
	}
}

func resultCell(result string) *tview.TableCell {
	display := result
	if display == "" {
		display = "running"
	}
	return tview.NewTableCell(" " + display).SetTextColor(resultColor(result))
}

func resultIcon(result string) string {
	switch result {
	case "SUCCESS":
		return "✓"
	case "FAILURE", "ERROR", "NODE_FAILURE":
		return "✗"
	case "SKIPPED":
		return "–"
	default:
		return "●"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
