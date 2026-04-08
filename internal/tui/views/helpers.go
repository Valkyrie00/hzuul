package views

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
)

var colorTableHeader = ColorAccent

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
		display = "RUNNING"
	}
	return coloredCell(" "+display, resultColor(result))
}

func coloredCell(text string, color tcell.Color) *tview.TableCell {
	cell := tview.NewTableCell(text).SetTextColor(color)
	cell.SetSelectedStyle(tcell.StyleDefault.Background(ColorSelectBg).Foreground(color))
	return cell
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

func formatBuildDuration(v any) string {
	if v == nil {
		return "—"
	}
	var secs float64
	switch d := v.(type) {
	case float64:
		secs = d
	case int:
		secs = float64(d)
	case json.Number:
		secs, _ = d.Float64()
	default:
		s := fmt.Sprintf("%v", v)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			secs = f
		} else {
			return s
		}
	}
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh %d min %d secs", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%d min %d secs", m, s)
	}
	return fmt.Sprintf("%d secs", s)
}

func openURL(u string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "cmd", []string{"/c", "start"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, u)
	exec.Command(cmd, args...).Start()
}

func handleBuildOpenKeys(event *tcell.EventKey, client *api.Client, build *api.Build) *tcell.EventKey {
	switch event.Rune() {
	case 'o':
		if build.UUID != "" && client != nil {
			openURL(client.BuildURL(build.UUID))
		}
		return nil
	case 'c':
		if build.Ref.RefURL != "" {
			openURL(build.Ref.RefURL)
		}
		return nil
	}
	return event
}

func renderBuildRows(table *tview.Table, builds []api.Build, primaryField func(api.Build) string) {
	muted := ColorMuted
	dim := ColorDim
	for i, b := range builds {
		row := i + 1
		rc := resultColor(b.Result)
		table.SetCell(row, 0, coloredCell(" "+resultIcon(b.Result)+" "+primaryField(b), rc))
		table.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(muted))
		table.SetCell(row, 2, tview.NewTableCell(" "+b.Pipeline).SetTextColor(muted))
		table.SetCell(row, 3, tview.NewTableCell(" "+formatChange(b.Ref)).SetTextColor(muted))
		table.SetCell(row, 4, resultCell(b.Result))
		table.SetCell(row, 5, tview.NewTableCell(" "+formatBuildDuration(b.Duration)).SetTextColor(muted))
		table.SetCell(row, 6, tview.NewTableCell(" "+formatTimestamp(b.StartTime)).SetTextColor(dim))
	}
	table.Select(1, 0)
}

func formatChange(ref api.BuildRef) string {
	if ref.Change == nil {
		return ""
	}
	c := fmt.Sprintf("%v", ref.Change)
	if c == "<nil>" || c == "" {
		return ""
	}
	if ref.Patchset != nil {
		p := fmt.Sprintf("%v", ref.Patchset)
		if p != "" && p != "<nil>" {
			return c + "," + p
		}
	}
	return c
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
