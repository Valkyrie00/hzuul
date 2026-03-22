package views

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type statusRow struct {
	pipeline string
	queue    string
	item     api.QueueItem
}

// rowEntry identifies what a table row represents.
type rowEntry struct {
	kind   string // "change" or "job"
	rowIdx int    // index in StatusView.rows
	jobIdx int    // index in rows[rowIdx].item.Jobs (only for kind=="job")
}

type StatusView struct {
	root     *tview.Flex
	table    *tview.Table
	logView  *BuildLogView
	pages    *tview.Pages
	app      *tview.Application
	client   *api.Client
	rowMap   map[int]rowEntry
	rows     []statusRow
	expanded map[int]bool // rowIdx → expanded
	status   *api.Status
}

func NewStatusView(app *tview.Application) *StatusView {
	bg := tcell.NewRGBColor(24, 24, 32)

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(bg)

	logView := NewBuildLogView(app)

	pages := tview.NewPages().
		AddPage("table", table, true, true).
		AddPage("log", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(bg)

	v := &StatusView{
		root:     root,
		table:    table,
		logView:  logView,
		pages:    pages,
		app:      app,
		rowMap:   make(map[int]rowEntry),
		expanded: make(map[int]bool),
	}

	table.SetSelectedFunc(func(row, _ int) {
		entry, ok := v.rowMap[row]
		if !ok {
			return
		}
		switch entry.kind {
		case "change":
			v.expanded[entry.rowIdx] = !v.expanded[entry.rowIdx]
			sel, _ := v.table.GetSelection()
			v.rebuildTable()
			if sel < v.table.GetRowCount() {
				v.table.Select(sel, 0)
			}
		case "job":
			v.showJobDetail(entry)
		}
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'l' {
			row, _ := table.GetSelection()
			entry, ok := v.rowMap[row]
			if !ok || entry.kind != "job" {
				return event
			}
			sr := v.rows[entry.rowIdx]
			job := sr.item.Jobs[entry.jobIdx]
			if job.UUID != "" && v.client != nil {
				v.streamJobLog(sr, job)
			}
			return nil
		}
		return event
	})

	logView.Root().(*tview.Flex).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.logView.Stop()
			v.pages.SwitchToPage("table")
			return nil
		}
		return event
	})

	return v
}

func (v *StatusView) Root() tview.Primitive { return v.root }

func (v *StatusView) Load(client *api.Client) {
	v.client = client
	v.table.Clear()
	v.setStatusHeader()
	v.rows = nil
	v.rowMap = make(map[int]rowEntry)

	go func() {
		status, err := client.GetStatus()
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("[red]Error: %v[-]", err)).SetExpansion(1))
				return
			}
			v.status = status
			v.collectRows(status)
			v.rebuildTable()
		})
	}()
}

func (v *StatusView) collectRows(status *api.Status) {
	v.rows = nil
	for _, p := range status.Pipelines {
		for _, q := range p.ChangeQueues {
			for _, heads := range q.Heads {
				for _, item := range heads {
					v.rows = append(v.rows, statusRow{
						pipeline: p.Name,
						queue:    q.Name,
						item:     item,
					})
				}
			}
		}
	}
}

const statusCols = 7

func (v *StatusView) setStatusHeader() {
	headers := []struct {
		name      string
		expansion int
	}{
		{"Project", 2},
		{"Change", 0},
		{"Owner", 0},
		{"Progress", 0},
		{"Elapsed", 0},
		{"ETA", 0},
		{"Jobs", 1},
	}
	for i, h := range headers {
		cell := tview.NewTableCell(h.name).
			SetTextColor(tcell.NewRGBColor(56, 132, 244)).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold).
			SetExpansion(h.expansion)
		v.table.SetCell(0, i, cell)
	}
}

func (v *StatusView) rebuildTable() {
	v.table.Clear()
	v.setStatusHeader()
	v.rowMap = make(map[int]rowEntry)

	sectionBg := tcell.NewRGBColor(30, 30, 42)
	jobBg := tcell.NewRGBColor(28, 28, 38)
	muted := tcell.NewRGBColor(120, 120, 140)
	accent := tcell.NewRGBColor(56, 132, 244)
	now := time.Now()
	tableRow := 1

	// Group rows by pipeline for section headers
	type pipelineGroup struct {
		name  string
		start int
		count int
	}
	var groups []pipelineGroup
	for i, sr := range v.rows {
		if len(groups) == 0 || groups[len(groups)-1].name != sr.pipeline {
			groups = append(groups, pipelineGroup{name: sr.pipeline, start: i, count: 1})
		} else {
			groups[len(groups)-1].count++
		}
	}

	for _, g := range groups {
		// Pipeline section header
		headerText := fmt.Sprintf("▸ %s (%d)", g.name, g.count)
		v.table.SetCell(tableRow, 0, tview.NewTableCell(headerText).
			SetTextColor(accent).SetAttributes(tcell.AttrBold).
			SetSelectable(false).SetBackgroundColor(sectionBg).SetExpansion(2))
		for col := 1; col < statusCols; col++ {
			exp := 0
			if col == statusCols-1 {
				exp = 1
			}
			v.table.SetCell(tableRow, col,
				tview.NewTableCell("").SetSelectable(false).SetBackgroundColor(sectionBg).SetExpansion(exp))
		}
		tableRow++

		for i := g.start; i < g.start+g.count; i++ {
			sr := v.rows[i]
			v.rowMap[tableRow] = rowEntry{kind: "change", rowIdx: i}

			running, success, failure, other := jobCounts(sr.item.Jobs)
			total := len(sr.item.Jobs)
			bar := compactProgress(running, success, failure, other, total)
			summary := jobSummaryText(running, success, failure, other)
			elapsed := formatElapsed(sr.item.EnqueueTime, now)
			eta := formatRemaining(sr.item.RemainingTime)

			project := sr.item.ProjectName()
			if project == "" {
				project = sr.queue
			}
			changeID := sr.item.ChangeID()
			owner := sr.item.Owner()

			arrow := "▸"
			if v.expanded[i] {
				arrow = "▾"
			}

			v.table.SetCell(tableRow, 0, tview.NewTableCell(fmt.Sprintf(" %s %s", arrow, project)).SetTextColor(tcell.ColorWhite).SetExpansion(2))
			v.table.SetCell(tableRow, 1, tview.NewTableCell("#"+changeID).SetTextColor(muted).SetExpansion(0))
			v.table.SetCell(tableRow, 2, tview.NewTableCell(owner).SetTextColor(tcell.NewRGBColor(180, 160, 220)).SetExpansion(0))
			v.table.SetCell(tableRow, 3, tview.NewTableCell(bar).SetExpansion(0))
			v.table.SetCell(tableRow, 4, tview.NewTableCell(elapsed).SetTextColor(muted).SetAlign(tview.AlignRight).SetExpansion(0))
			v.table.SetCell(tableRow, 5, tview.NewTableCell(eta).SetTextColor(muted).SetAlign(tview.AlignRight).SetExpansion(0))
			v.table.SetCell(tableRow, 6, tview.NewTableCell(summary).SetTextColor(muted).SetExpansion(1))
			tableRow++

			if v.expanded[i] {
				for j, job := range sr.item.Jobs {
					v.rowMap[tableRow] = rowEntry{kind: "job", rowIdx: i, jobIdx: j}

					result := "running"
					if job.Result != nil {
						result = *job.Result
					}
					icon := jobIcon(result)
					timeStr := jobTimeInfo(job, now)
					nv := ""
					if !job.Voting {
						nv = " [::d](nv)[-]"
					}
					jobText := fmt.Sprintf("      %s %s%s%s", icon, job.Name, nv, timeStr)
					resultText := jobResultText(result)

					v.table.SetCell(tableRow, 0, tview.NewTableCell(jobText).SetTextColor(tcell.ColorWhite).SetExpansion(2).SetBackgroundColor(jobBg))
					for col := 1; col < statusCols-1; col++ {
						v.table.SetCell(tableRow, col, tview.NewTableCell("").SetExpansion(0).SetBackgroundColor(jobBg))
					}
					v.table.SetCell(tableRow, statusCols-1, tview.NewTableCell(resultText).SetExpansion(1).SetBackgroundColor(jobBg))
					tableRow++
				}
			}
		}
	}

	if len(v.rows) == 0 {
		v.table.SetCell(tableRow, 0, tview.NewTableCell("[::d]All pipelines idle[-]").SetExpansion(1))
	}
}

func (v *StatusView) showJobDetail(entry rowEntry) {
	sr := v.rows[entry.rowIdx]
	job := sr.item.Jobs[entry.jobIdx]
	now := time.Now()

	result := "running"
	if job.Result != nil {
		result = *job.Result
	}

	text := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	text.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))
	text.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", job.Name)).
		SetBorderColor(tcell.NewRGBColor(60, 60, 80))

	fmt.Fprintf(text, "[bold]Job:[-]       %s\n", job.Name)
	fmt.Fprintf(text, "[bold]Result:[-]    %s\n", jobResultText(result))
	fmt.Fprintf(text, "[bold]Pipeline:[-]  %s\n", sr.pipeline)
	fmt.Fprintf(text, "[bold]Project:[-]   %s\n", sr.item.ProjectName())
	fmt.Fprintf(text, "[bold]Change:[-]    #%s\n", sr.item.ChangeID())
	if job.UUID != "" {
		fmt.Fprintf(text, "[bold]UUID:[-]      %s\n", job.UUID)
	}
	if !job.Voting {
		fmt.Fprintf(text, "[bold]Voting:[-]    [yellow]non-voting[-]\n")
	}
	elapsed := jobTimeInfo(job, now)
	if elapsed != "" {
		fmt.Fprintf(text, "[bold]Time:[-]      %s\n", strings.TrimSpace(elapsed))
	}
	if job.ReportURL != "" {
		fmt.Fprintf(text, "[bold]Report:[-]    %s\n", job.ReportURL)
	}

	fmt.Fprintln(text)
	if job.UUID != "" {
		fmt.Fprintln(text, "[::d]Press l to stream log, q/Esc to close[-]")
	} else {
		fmt.Fprintln(text, "[::d]Press q/Esc to close[-]")
	}

	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'l' && job.UUID != "" && v.client != nil {
			v.pages.RemovePage("jobdetail")
			v.streamJobLog(sr, job)
			return nil
		}
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.pages.RemovePage("jobdetail")
			return nil
		}
		return event
	})

	v.pages.AddAndSwitchToPage("jobdetail", center(text, 70, 18), true)
}

func (v *StatusView) streamJobLog(sr statusRow, job api.JobStatus) {
	v.logView.Stop()

	v.logView.header.Clear()
	project := sr.item.ProjectName()
	if project == "" {
		project = sr.queue
	}
	fmt.Fprintf(v.logView.header, " [bold]Log[-] │ [blue]%s[-] │ %s │ #%s",
		job.Name, project, sr.item.ChangeID())

	v.logView.textView.Clear()
	fmt.Fprintln(v.logView.textView, "[::d]Connecting to log stream...[-]")

	v.logView.stopCh = make(chan struct{})
	v.pages.SwitchToPage("log")

	go func() {
		uuid, logfile := parseJobStreamURL(job)
		if uuid == "" {
			v.app.QueueUpdateDraw(func() {
				v.logView.textView.Clear()
				fmt.Fprintln(v.logView.textView, "[red]No stream UUID available for this job[-]")
			})
			return
		}
		streamer, err := v.client.StreamLog(uuid, logfile)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.logView.textView.Clear()
				fmt.Fprintf(v.logView.textView, "[red]Stream error: %v[-]\n\n", err)
				if job.ReportURL != "" {
					fmt.Fprintf(v.logView.textView, "[::d]Report URL: %s[-]\n", job.ReportURL)
				}
			})
			return
		}

		v.logView.mu.Lock()
		v.logView.streamer = streamer
		v.logView.mu.Unlock()

		v.app.QueueUpdateDraw(func() {
			v.logView.textView.Clear()
		})

		// Buffer incoming messages and flush to the view periodically
		// to avoid per-line redraws during the initial log dump.
		var buf strings.Builder
		var bufMu sync.Mutex
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				msg, err := streamer.ReadMessage()
				if err != nil {
					bufMu.Lock()
					buf.WriteString("\n[::d]--- stream ended ---[-]\n")
					bufMu.Unlock()
					return
				}
				bufMu.Lock()
				buf.WriteString(msg)
				bufMu.Unlock()
			}
		}()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-v.logView.stopCh:
				return
			case <-done:
				bufMu.Lock()
				remaining := buf.String()
				buf.Reset()
				bufMu.Unlock()
				if remaining != "" {
					v.app.QueueUpdateDraw(func() {
						fmt.Fprint(v.logView.textView, remaining)
						v.logView.textView.ScrollToEnd()
					})
				}
				return
			case <-ticker.C:
				bufMu.Lock()
				chunk := buf.String()
				buf.Reset()
				bufMu.Unlock()
				if chunk != "" {
					v.app.QueueUpdateDraw(func() {
						fmt.Fprint(v.logView.textView, chunk)
						v.logView.textView.ScrollToEnd()
					})
				}
			}
		}
	}()
}

func jobResultText(result string) string {
	switch result {
	case "SUCCESS":
		return "[green]SUCCESS[-]"
	case "FAILURE", "ERROR", "RETRY_LIMIT":
		return "[red]" + result + "[-]"
	case "LOST", "ABORTED", "DISK_FULL", "TIMED_OUT":
		return "[yellow]" + result + "[-]"
	case "running":
		return "[blue]running[-]"
	default:
		return "[gray]" + result + "[-]"
	}
}

// parseJobStreamURL extracts the UUID and logfile from a JobStatus.
// The job's URL field has the form "stream/<uuid>?logfile=console.log".
// Falls back to job.UUID with default "console.log".
func parseJobStreamURL(job api.JobStatus) (uuid, logfile string) {
	if job.URL != "" {
		u, err := url.Parse(job.URL)
		if err == nil {
			uuid = path.Base(u.Path)
			logfile = u.Query().Get("logfile")
		}
	}
	if uuid == "" {
		uuid = job.UUID
	}
	if logfile == "" {
		logfile = "console.log"
	}
	return
}

// center wraps a primitive in a centered flex layout (used for popups).
func center(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}

func jobCounts(jobs []api.JobStatus) (running, success, failure, other int) {
	for _, j := range jobs {
		if j.Result == nil {
			running++
			continue
		}
		switch *j.Result {
		case "SUCCESS":
			success++
		case "FAILURE", "ERROR", "RETRY_LIMIT":
			failure++
		default:
			other++
		}
	}
	return
}

func compactProgress(running, success, failure, other, total int) string {
	if total == 0 {
		return "[::d]—[-]"
	}
	const width = 10
	var b strings.Builder
	segments := []struct {
		count int
		color string
	}{
		{success, "[green]"},
		{failure, "[red]"},
		{running, "[blue]"},
		{other, "[yellow]"},
	}
	filled := 0
	for _, s := range segments {
		cells := int(math.Round(float64(s.count) * width / float64(total)))
		if filled+cells > width {
			cells = width - filled
		}
		for range cells {
			b.WriteString(s.color + "\u2588[-]")
		}
		filled += cells
	}
	for range width - filled {
		b.WriteString("[::d]\u2591[-]")
	}
	return b.String()
}

func jobSummaryText(running, success, failure, other int) string {
	var parts []string
	if success > 0 {
		parts = append(parts, fmt.Sprintf("[green]%d ok[-]", success))
	}
	if failure > 0 {
		parts = append(parts, fmt.Sprintf("[red]%d fail[-]", failure))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("[blue]%d run[-]", running))
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("[yellow]%d other[-]", other))
	}
	if len(parts) == 0 {
		return "[::d]no jobs[-]"
	}
	return strings.Join(parts, " / ")
}

func jobIcon(result string) string {
	switch result {
	case "SUCCESS":
		return "[green]\u2714[-]"
	case "FAILURE", "ERROR", "RETRY_LIMIT":
		return "[red]\u2718[-]"
	case "LOST", "ABORTED", "DISK_FULL", "TIMED_OUT":
		return "[yellow]\u26a0[-]"
	case "running":
		return "[blue]\u25cf[-]"
	default:
		return "[gray]\u25cb[-]"
	}
}

func formatElapsed(enqueueTime interface{ String() string }, now time.Time) string {
	s := enqueueTime.String()
	if s == "" || s == "0" {
		return "-"
	}
	var ms float64
	if _, err := fmt.Sscanf(s, "%f", &ms); err != nil || ms == 0 {
		return "-"
	}
	t := time.UnixMilli(int64(ms))
	d := now.Sub(t)
	if d < 0 {
		return "-"
	}
	return formatDuration(d)
}

func formatRemaining(remainingTime interface{ String() string }) string {
	s := remainingTime.String()
	if s == "" || s == "0" || s == "null" {
		return "-"
	}
	var ms float64
	if _, err := fmt.Sscanf(s, "%f", &ms); err != nil || ms <= 0 {
		return "-"
	}
	return formatDuration(time.Duration(int64(ms)) * time.Millisecond)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1m"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func jobTimeInfo(job api.JobStatus, now time.Time) string {
	if job.Result != nil {
		elapsed := job.ElapsedTime.String()
		if elapsed != "" && elapsed != "0" {
			var ms float64
			if _, err := fmt.Sscanf(elapsed, "%f", &ms); err == nil && ms > 0 {
				return fmt.Sprintf(" [::d]%s[-]", formatDuration(time.Duration(int64(ms))*time.Millisecond))
			}
		}
		return ""
	}
	s := job.StartTime.String()
	if s == "" || s == "0" {
		return ""
	}
	var ms float64
	if _, err := fmt.Sscanf(s, "%f", &ms); err != nil || ms == 0 {
		return ""
	}
	d := now.Sub(time.UnixMilli(int64(ms)))
	if d < 0 {
		return ""
	}
	rem := job.RemainingTime.String()
	if rem != "" && rem != "0" {
		var rms float64
		if _, err := fmt.Sscanf(rem, "%f", &rms); err == nil && rms > 0 {
			return fmt.Sprintf(" [::d]%s (ETA %s)[-]", formatDuration(d), formatDuration(time.Duration(int64(rms))*time.Millisecond))
		}
	}
	return fmt.Sprintf(" [::d]%s[-]", formatDuration(d))
}
