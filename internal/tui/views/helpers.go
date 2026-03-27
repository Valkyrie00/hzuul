package views

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
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

const defaultPageSize = 50

func parseBuildFilter(text string) api.BuildFilter {
	f := api.BuildFilter{Limit: defaultPageSize}
	if text == "" {
		return f
	}
	if idx := strings.Index(text, ":"); idx > 0 {
		prefix := strings.ToLower(text[:idx])
		value := strings.TrimSpace(text[idx+1:])
		switch prefix {
		case "job":
			f.JobName = value
		case "project":
			f.Project = value
		case "pipeline":
			f.Pipeline = value
		case "branch":
			f.Branch = value
		case "result":
			f.Result = value
		case "change":
			f.Change = value
		default:
			f.JobName = text
		}
		return f
	}
	if strings.Contains(text, "/") {
		f.Project = text
	} else {
		f.JobName = text
	}
	return f
}

func parseBuildsetFilter(text string) api.BuildFilter {
	f := api.BuildFilter{Limit: defaultPageSize}
	if text == "" {
		return f
	}
	if idx := strings.Index(text, ":"); idx > 0 {
		prefix := strings.ToLower(text[:idx])
		value := strings.TrimSpace(text[idx+1:])
		switch prefix {
		case "project":
			f.Project = value
		case "pipeline":
			f.Pipeline = value
		case "branch":
			f.Branch = value
		case "result":
			f.Result = value
		case "change":
			f.Change = value
		default:
			f.Pipeline = text
		}
		return f
	}
	if strings.Contains(text, "/") {
		f.Project = text
	} else {
		f.Pipeline = text
	}
	return f
}
