package views

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
)

type statusRow struct {
	pipeline string
	queue    string
	item     api.QueueItem
}

// rowEntry identifies what a table row represents.
type rowEntry struct {
	kind     string // "pipeline", "change" or "job"
	rowIdx   int    // index in StatusView.rows (for "change"/"job")
	jobIdx   int    // index in rows[rowIdx].item.Jobs (only for kind=="job")
	pipeline string // pipeline name (only for kind=="pipeline")
}

type StatusView struct {
	root               *tview.Flex
	table              *tview.Table
	logView            *BuildLogView
	pages              *tview.Pages
	app                *tview.Application
	client             *api.Client
	rowMap             map[int]rowEntry
	rows               []statusRow
	expanded           map[int]bool    // rowIdx → expanded
	collapsedPipelines map[string]bool // pipeline name → collapsed
	status             *api.Status
	filter             string
}

func NewStatusView(app *tview.Application, dlManager *DownloadManager) *StatusView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(ColorSelectBg))

	logView := NewBuildLogView(app, dlManager)
	keys := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(ColorNavBg)
	fmt.Fprint(keys, " [#3884f4]enter[-:-:-][::d]:expand/open[-:-:-]  [#3884f4]o[-:-:-][::d]:open web[-:-:-]  [#3884f4]c[-:-:-][::d]:change[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]")

	tableWithKeys := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	tableWithKeys.SetBackgroundColor(ColorBg)

	pages := tview.NewPages().
		AddPage("table", tableWithKeys, true, true).
		AddPage("log", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(ColorBg)

	v := &StatusView{
		root:               root,
		table:              table,
		logView:            logView,
		pages:              pages,
		app:                app,
		rowMap:             make(map[int]rowEntry),
		expanded:           make(map[int]bool),
		collapsedPipelines: make(map[string]bool),
	}

	table.SetSelectedFunc(func(row, _ int) {
		entry, ok := v.rowMap[row]
		if !ok {
			return
		}
		switch entry.kind {
		case "pipeline":
			pname := entry.pipeline
			v.collapsedPipelines[pname] = !v.collapsedPipelines[pname]
			v.rebuildTable()
			for r, e := range v.rowMap {
				if e.kind == "pipeline" && e.pipeline == pname {
					v.table.Select(r, 0)
					break
				}
			}
		case "change":
			sr := v.rows[entry.rowIdx]
			if len(sr.item.Jobs) == 0 {
				if u := sr.item.ChangeURL(); u != "" {
					openURL(u)
				}
				return
			}
			v.expanded[entry.rowIdx] = !v.expanded[entry.rowIdx]
			sel, _ := v.table.GetSelection()
			v.rebuildTable()
			if sel < v.table.GetRowCount() {
				v.table.Select(sel, 0)
			}
		case "job":
			sr := v.rows[entry.rowIdx]
			job := sr.item.Jobs[entry.jobIdx]
			if job.Result == nil && job.UUID != "" {
				v.streamJobLog(sr, job)
			} else if job.UUID != "" {
				v.showBuildDetail(sr, job)
			} else {
				changeURL := sr.item.ChangeURL()
				if changeURL != "" {
					openURL(changeURL)
				}
			}
		}
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := table.GetSelection()
		entry, ok := v.rowMap[row]

		if event.Rune() == 'o' {
			if !ok {
				return event
			}
			if entry.kind == "job" {
				sr := v.rows[entry.rowIdx]
				job := sr.item.Jobs[entry.jobIdx]
				if job.UUID != "" && v.client != nil {
					openURL(v.client.BuildURL(job.UUID))
				}
			}
			return nil
		}

		if event.Rune() == 'c' {
			if !ok {
				return event
			}
			changeURL := v.rows[entry.rowIdx].item.ChangeURL()
			if changeURL != "" {
				openURL(changeURL)
			}
			return nil
		}

		return event
	})

	logView.SetBackHandler(func() {
		v.pages.SwitchToPage("table")
	})

	return v
}

func (v *StatusView) SetBookmarkManager(bm *BookmarkManager) { v.logView.SetBookmarkManager(bm) }
func (v *StatusView) Root() tview.Primitive                   { return v.root }

func (v *StatusView) SetFilter(term string) {
	v.filter = term
	v.rebuildTable()
	if v.table.GetRowCount() > 1 {
		v.table.Select(1, 0)
	}
}

func (v *StatusView) Load(client *api.Client) {
	v.client = client

	if v.status == nil {
		v.table.Clear()
		v.setStatusHeader()
		v.table.SetCell(1, 0, tview.NewTableCell(" [yellow]Loading...[-]").SetSelectable(false).SetExpansion(1))
	}

	go func() {
		status, err := client.GetStatus()
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.table.Clear()
				v.setStatusHeader()
				v.rows = nil
				v.rowMap = make(map[int]rowEntry)
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)).SetExpansion(1))
				return
			}

			sel, _ := v.table.GetSelection()
			firstLoad := v.status == nil

			v.status = status
			v.collectRows(status)
			v.rebuildTable()

			if sel >= v.table.GetRowCount() {
				sel = 1
			}
			if sel < 1 {
				sel = 1
			}
			v.table.Select(sel, 0)
			if firstLoad {
				v.table.ScrollToBeginning()
			}
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

const statusCols = 6

func (v *StatusView) setStatusHeader() {
	headers := []struct {
		name      string
		expansion int
	}{
		{"Project", 1},
		{"Change", 0},
		{"Owner", 0},
		{"Progress", 0},
		{"Elapsed", 0},
		{"Jobs", 0},
	}
	for i, h := range headers {
		cell := tview.NewTableCell(" " + h.name).
			SetTextColor(ColorAccent).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold).
			SetExpansion(h.expansion)
		v.table.SetCell(0, i, cell)
	}
}

func (v *StatusView) statusRowMatchesFilter(sr statusRow) bool {
	if v.filter == "" {
		return true
	}
	project := sr.item.ProjectName()
	if project == "" {
		project = sr.queue
	}
	fields := []string{project, sr.item.ChangeID(), sr.item.Owner(), sr.pipeline}
	for _, j := range sr.item.Jobs {
		fields = append(fields, j.Name)
	}
	return rowMatchesFilter(v.filter, fields...)
}

func (v *StatusView) rebuildTable() {
	v.table.Clear()
	v.setStatusHeader()
	v.rowMap = make(map[int]rowEntry)

	sectionBg := ColorBg
	jobBg := ColorBg
	muted := ColorMuted
	sectionColor := tcell.NewRGBColor(230, 185, 90)
	now := time.Now()
	tableRow := 1 // row 0 is header

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

	sectionsRendered := 0
	for _, g := range groups {
		var matchingIndices []int
		for i := g.start; i < g.start+g.count; i++ {
			if v.statusRowMatchesFilter(v.rows[i]) {
				matchingIndices = append(matchingIndices, i)
			}
		}
		if len(matchingIndices) == 0 {
			continue
		}

		if sectionsRendered > 0 {
			for col := 0; col < statusCols; col++ {
				exp := 0
				if col == 0 {
					exp = 1
				}
				v.table.SetCell(tableRow, col,
					tview.NewTableCell("").SetExpansion(exp))
			}
			tableRow++
		}

		collapsed := v.collapsedPipelines[g.name]
		arrow := "▾"
		if collapsed {
			arrow = "▸"
		}
		headerText := fmt.Sprintf(" %s %s (%d) ", arrow, g.name, len(matchingIndices))
		v.rowMap[tableRow] = rowEntry{kind: "pipeline", pipeline: g.name}
		v.table.SetCell(tableRow, 0, tview.NewTableCell(headerText).
			SetTextColor(sectionColor).SetAttributes(tcell.AttrBold).
			SetBackgroundColor(sectionBg).SetExpansion(1).SetMaxWidth(45))
		for col := 1; col < statusCols; col++ {
			v.table.SetCell(tableRow, col,
				tview.NewTableCell("").SetBackgroundColor(sectionBg))
		}
		tableRow++
		sectionsRendered++

		if collapsed {
			continue
		}

		for _, i := range matchingIndices {
			sr := v.rows[i]
			v.rowMap[tableRow] = rowEntry{kind: "change", rowIdx: i}

			project := sr.item.ProjectName()
			if project == "" {
				project = sr.queue
			}
			changeID := sr.item.ChangeID()
			owner := sr.item.Owner()
			total := len(sr.item.Jobs)

			displayID := changeID
			if len(displayID) > 12 {
				displayID = displayID[:8]
			}

			if total == 0 {
				label := "[::d]queued[-]"
				if !sr.item.Live {
					label = "[::d]dependency[-]"
				}
				v.table.SetCell(tableRow, 0, tview.NewTableCell(fmt.Sprintf("   ℹ %s", project)).SetTextColor(muted).SetExpansion(1))
				v.table.SetCell(tableRow, 1, tview.NewTableCell(" #"+displayID).SetTextColor(muted).SetExpansion(0))
				v.table.SetCell(tableRow, 2, tview.NewTableCell(" "+owner).SetTextColor(muted).SetExpansion(0))
				v.table.SetCell(tableRow, 3, tview.NewTableCell("").SetExpansion(0))
				elapsed := formatElapsed(sr.item.EnqueueTime, now)
				v.table.SetCell(tableRow, 4, tview.NewTableCell(elapsed+" ").SetTextColor(muted).SetAlign(tview.AlignRight).SetExpansion(0))
				v.table.SetCell(tableRow, 5, tview.NewTableCell(" "+label).SetTextColor(muted))
				tableRow++
				continue
			}

			running, success, failure, other := jobCounts(sr.item.Jobs)
			bar := compactProgress(running, success, failure, other, total)
			summary := jobSummaryText(running, success, failure, other)
			elapsed := formatElapsed(sr.item.EnqueueTime, now)

			arrow := "▸"
			if v.expanded[i] {
				arrow = "▾"
			}

			v.table.SetCell(tableRow, 0, tview.NewTableCell(fmt.Sprintf("   %s %s", arrow, project)).SetTextColor(tcell.ColorWhite).SetExpansion(1))
			v.table.SetCell(tableRow, 1, tview.NewTableCell(" #"+displayID).SetTextColor(muted).SetExpansion(0))
			v.table.SetCell(tableRow, 2, tview.NewTableCell(" "+owner).SetTextColor(tcell.NewRGBColor(180, 160, 220)).SetExpansion(0))
			v.table.SetCell(tableRow, 3, tview.NewTableCell(" "+bar).SetExpansion(0))
			v.table.SetCell(tableRow, 4, tview.NewTableCell(elapsed+" ").SetTextColor(muted).SetAlign(tview.AlignRight).SetExpansion(0))
			v.table.SetCell(tableRow, 5, tview.NewTableCell(" "+summary).SetTextColor(muted))
			tableRow++

			if v.expanded[i] {
				for j, job := range sr.item.Jobs {
					v.rowMap[tableRow] = rowEntry{kind: "job", rowIdx: i, jobIdx: j}

					result := "running"
					if job.Result != nil {
						result = *job.Result
					}
					nameColor := jobNameColor(result)
					nv := ""
					if !job.Voting {
						nv = " (nv)"
					}
					jobElapsed, _ := jobTimeParts(job, now)
					resultText := jobResultText(result)

					v.table.SetCell(tableRow, 0, tview.NewTableCell("       "+jobIcon(result)+" "+job.Name+nv).SetTextColor(nameColor).SetExpansion(1).SetBackgroundColor(jobBg))
					v.table.SetCell(tableRow, 1, tview.NewTableCell("").SetBackgroundColor(jobBg))
					v.table.SetCell(tableRow, 2, tview.NewTableCell("").SetBackgroundColor(jobBg))
					v.table.SetCell(tableRow, 3, tview.NewTableCell("").SetBackgroundColor(jobBg))
					v.table.SetCell(tableRow, 4, tview.NewTableCell(jobElapsed+" ").SetTextColor(muted).SetAlign(tview.AlignRight).SetBackgroundColor(jobBg))
					v.table.SetCell(tableRow, 5, tview.NewTableCell(" "+resultText).SetBackgroundColor(jobBg))
					tableRow++
				}
			}
		}
	}

	if len(v.rows) == 0 {
		v.table.SetCell(tableRow, 0, tview.NewTableCell(" [::d]All pipelines idle[-]").SetExpansion(1))
	} else if tableRow == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetExpansion(1))
	}
}

func (v *StatusView) streamJobLog(sr statusRow, job api.JobStatus) {
	project := sr.item.ProjectName()
	if project == "" {
		project = sr.queue
	}
	uuid, _ := parseJobStreamURL(job)
	if uuid == "" {
		uuid = job.UUID
	}
	build := &api.Build{
		UUID:    uuid,
		JobName: job.Name,
		Ref: api.BuildRef{
			Project: project,
			Branch:  sr.item.ChangeID(),
			RefURL:  sr.item.ChangeURL(),
		},
	}
	v.logView.StreamBuild(v.client, build)
	v.pages.SwitchToPage("log")
}

func (v *StatusView) showBuildDetail(sr statusRow, job api.JobStatus) {
	v.logView.Stop()
	v.logView.header.Clear()
	project := sr.item.ProjectName()
	if project == "" {
		project = sr.queue
	}
	fmt.Fprintf(v.logView.header, " [bold]Build Detail[-] │ [#3884f4]%s[-] │ %s │ #%s",
		job.Name, project, sr.item.ChangeID())

	v.logView.textView.Clear()
	fmt.Fprintln(v.logView.textView, "[::d]Loading build detail...[-]")
	v.pages.SwitchToPage("log")

	go func() {
		build, err := v.client.GetBuild(job.UUID)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.logView.textView.Clear()
				fmt.Fprintf(v.logView.textView, "[red]Error loading build: %v[-]\n\n", err)
				fmt.Fprintf(v.logView.textView, "[bold]Job:[-]       %s\n", job.Name)
				fmt.Fprintf(v.logView.textView, "[bold]Pipeline:[-]  %s\n", sr.pipeline)
				fmt.Fprintf(v.logView.textView, "[bold]Change:[-]    #%s\n", sr.item.ChangeID())
				if job.ReportURL != "" {
					fmt.Fprintf(v.logView.textView, "\n[::d]Report: %s[-]\n", job.ReportURL)
				}
				return
			}
			v.logView.ShowStaticLog(v.client, build)
		})
	}()
}

func jobResultText(result string) string {
	switch result {
	case "SUCCESS":
		return "[#48c78e]SUCCESS[-]"
	case "FAILURE", "ERROR", "NODE_FAILURE":
		return "[#eb5757]" + result + "[-]"
	case "RETRY_LIMIT", "LOST", "ABORTED", "DISK_FULL", "TIMED_OUT":
		return "[#f2c94c]" + result + "[-]"
	case "SKIPPED":
		return "[#78788c]SKIPPED[-]"
	case "running":
		return "[#3884f4]RUNNING[-]"
	default:
		return "[#78788c]" + result + "[-]"
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
		{running, "[#3884f4]"},
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
		parts = append(parts, fmt.Sprintf("[#48c78e]%d[-:-:-]", success))
	}
	if failure > 0 {
		parts = append(parts, fmt.Sprintf("[#eb5757]%d[-:-:-]", failure))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("[#3884f4]%d[-:-:-]", running))
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("[#f2c94c]%d[-:-:-]", other))
	}
	if len(parts) == 0 {
		return "[#78788c]—[-:-:-]"
	}
	return strings.Join(parts, "[#5a5a6e]/[-:-:-]")
}

func jobNameColor(result string) tcell.Color {
	if result == "running" {
		return tcell.NewRGBColor(80, 160, 255)
	}
	return resultColor(result)
}

func jobIcon(result string) string {
	if result == "running" {
		return "●"
	}
	return resultIcon(result)
}

func jobTimeParts(job api.JobStatus, now time.Time) (elapsed, eta string) {
	if s := job.ElapsedTime.String(); s != "" && s != "0" {
		var ms float64
		if _, err := fmt.Sscanf(s, "%f", &ms); err == nil && ms > 0 {
			elapsed = formatDuration(time.Duration(int64(ms)) * time.Millisecond)
		}
	} else if s := job.StartTime.String(); s != "" && s != "0" {
		var sec float64
		if _, err := fmt.Sscanf(s, "%f", &sec); err == nil && sec > 0 {
			t := time.Unix(int64(sec), 0)
			d := now.Sub(t)
			if d > 0 {
				elapsed = formatDuration(d)
			}
		}
	}

	if job.Result == nil {
		if s := job.RemainingTime.String(); s != "" && s != "0" {
			var ms float64
			if _, err := fmt.Sscanf(s, "%f", &ms); err == nil && ms > 0 {
				eta = formatDuration(time.Duration(int64(ms)) * time.Millisecond)
			}
		}
	}
	return elapsed, eta
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

