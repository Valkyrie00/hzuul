package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

var (
	colorTableHeader = ColorAccent
	colorSeparator   = ColorSep
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

func formatTimestamp(ts string) string {
	if ts == "" {
		return "—"
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000000",
	} {
		if t, err := time.Parse(layout, ts); err == nil {
			now := time.Now()
			diff := now.Sub(t)
			switch {
			case diff < time.Minute:
				return "just now"
			case diff < time.Hour:
				return fmt.Sprintf("%dm ago", int(diff.Minutes()))
			case diff < 24*time.Hour:
				return fmt.Sprintf("%dh %dm ago", int(diff.Hours()), int(diff.Minutes())%60)
			case diff < 7*24*time.Hour:
				return t.Format("Mon 15:04")
			default:
				return t.Format("02 Jan 15:04")
			}
		}
	}
	if len(ts) > 16 {
		return ts[:16]
	}
	return ts
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
