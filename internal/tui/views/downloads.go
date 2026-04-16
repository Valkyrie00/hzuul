package views

import (
	"fmt"
	"time"

	"github.com/Valkyrie00/hzuul/internal/ai"
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type DownloadsView struct {
	pages    *tview.Pages
	root     *tview.Flex
	table    *tview.Table
	keys     *tview.TextView
	app      *tview.Application
	manager  *DownloadManager
	analysis *AnalysisPanel
}

func NewDownloadsView(app *tview.Application, manager *DownloadManager, aiCfg config.AIConfig) *DownloadsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	keys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(ColorNavBg)

	tableLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	tableLayout.SetBackgroundColor(ColorBg)

	panel := NewAnalysisPanel(app, aiCfg)

	pages := tview.NewPages().
		AddPage("table", tableLayout, true, true).
		AddPage("analysis", panel.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(ColorBg)

	v := &DownloadsView{
		pages:    pages,
		root:     root,
		table:    table,
		keys:     keys,
		app:      app,
		manager:  manager,
		analysis: panel,
	}

	v.updateKeys()

	panel.SetOnExit(func() {
		v.pages.SwitchToPage("table")
		v.app.SetFocus(v.table)
	})

	manager.SetOnChange(func() {
		v.renderTable()
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		rec := v.selectedRecord()
		if rec == nil {
			return event
		}
		switch {
		case event.Rune() == 'a':
			if rec.Status != DLDownloading {
				v.startAnalysis(rec)
			}
			return nil
		case event.Rune() == 'o':
			if rec.DestDir != "" {
				openURL(rec.DestDir)
			}
			return nil
		case event.Rune() == 'x':
			if rec.Status == DLDownloading {
				manager.Cancel(rec.UUID)
			}
			return nil
		case event.Rune() == 'd' || event.Key() == tcell.KeyDelete || event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2:
			if rec.Status != DLDownloading {
				manager.Remove(rec.UUID)
			}
			return nil
		}
		return event
	})

	root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if v.analysis.IsActive() {
			return v.analysis.HandleKey(event)
		}
		return event
	})

	return v
}

func (v *DownloadsView) Root() tview.Primitive { return v.root }

func (v *DownloadsView) Load(_ *api.Client) {
	v.renderTable()
	if !v.analysis.IsActive() {
		v.app.SetFocus(v.table)
	}
}

func (v *DownloadsView) SetFilter(_ string) {}

func (v *DownloadsView) IsModal() bool { return v.analysis.IsActive() }

func (v *DownloadsView) updateKeys() {
	v.keys.Clear()
	_, _ = fmt.Fprint(v.keys, " [#3884f4]o[-:-:-][::d]:open dir[-:-:-]  [#3884f4]x[-:-:-][::d]:cancel[-:-:-]  [#3884f4]d[-:-:-][::d]:delete[-:-:-]  [#e5c07b]a[-:-:-][::d]:AI analysis[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]")
}

func (v *DownloadsView) selectedRecord() *DownloadRecord {
	row, _ := v.table.GetSelection()
	idx := row - 1
	records := v.manager.Records()
	if idx < 0 || idx >= len(records) {
		return nil
	}
	return &records[idx]
}

func (v *DownloadsView) startAnalysis(rec *DownloadRecord) {
	v.analysis.Start(AnalysisFull, rec.JobName, rec.Project)
	v.pages.SwitchToPage("analysis")
	v.app.SetFocus(v.analysis.Content())

	_, _ = fmt.Fprint(v.analysis.Content(), "[::d]  Reading downloaded logs...[-:-:-]\n")

	go func() {
		da, err := ai.ReadLogsFromDir(rec.DestDir)
		v.app.QueueUpdateDraw(func() {
			if !v.analysis.IsActive() {
				return
			}
			if err != nil {
				_, _ = fmt.Fprintf(v.analysis.Content(), "\n[red]  Error reading logs: %v[-]\n", err)
				return
			}
			v.showAnalysisResults(rec, da)
		})
	}()
}

func (v *DownloadsView) showAnalysisResults(rec *DownloadRecord, da *ai.DirAnalysis) {
	w := v.analysis.Content()
	w.Clear()

	pbSummaries := ai.PlaybookSummaries(da.JobOutput)
	classification := ai.ClassifyFailure("FAILURE", da.FailedTasks, pbSummaries)
	phase := ai.DetermineFailurePhase(pbSummaries)

	v.analysis.WriteClassification(classification, phase)
	_, _ = fmt.Fprintf(w, "  [bold]Log files:[-]   %d snippets analyzed\n", len(da.LogFiles))

	input := ai.DirAnalysisInput{
		JobName: rec.JobName,
		Project: rec.Project,
		DestDir: rec.DestDir,
	}
	systemPrompt := ai.GetSystemPrompt()
	userPrompt := ai.BuildDirAnalysisPrompt(input, da)
	v.analysis.StartAI(systemPrompt, userPrompt)
}

func (v *DownloadsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Job", "Project", "Build", "Instance", "Status", "Progress", "Size", "Date")

	records := v.manager.Records()
	if len(records) == 0 {
		v.table.SetCell(1, 0, tview.NewTableCell(" [::d]No downloads yet. Press d in a build detail to start one.[-:-:-]").SetSelectable(false))
		return
	}

	muted := ColorMuted
	dim := ColorDim
	for i, r := range records {
		row := i + 1

		statusCell := statusCellForDL(r.Status)
		progressText := progressTextForDL(r)
		sizeText := ""
		if r.TotalBytes > 0 {
			sizeText = FormatBytes(r.TotalBytes)
		}
		dateText := formatDLDate(r.StartedAt)

		host := r.Host
		if host == "" {
			host = "—"
		}

		shortUUID := r.UUID
		if len(shortUUID) > 8 {
			shortUUID = shortUUID[len(shortUUID)-8:]
		}

		v.table.SetCell(row, 0, tview.NewTableCell(" "+r.JobName).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+r.Project).SetTextColor(muted).SetMaxWidth(35))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+shortUUID).SetTextColor(dim))
		v.table.SetCell(row, 3, tview.NewTableCell(" "+host).SetTextColor(dim).SetMaxWidth(30))
		v.table.SetCell(row, 4, statusCell)
		v.table.SetCell(row, 5, tview.NewTableCell(" "+progressText).SetTextColor(muted))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+sizeText).SetTextColor(muted))
		v.table.SetCell(row, 7, tview.NewTableCell(" "+dateText).SetTextColor(dim))
	}
}

func statusCellForDL(s DLStatus) *tview.TableCell {
	var text string
	var color tcell.Color
	switch s {
	case DLCompleted:
		text, color = "COMPLETED", tcell.ColorGreen
	case DLPartial:
		text, color = "PARTIAL", tcell.ColorOrange
	case DLFailed:
		text, color = "FAILED", tcell.ColorRed
	case DLCancelled:
		text, color = "CANCELLED", tcell.ColorYellow
	case DLDownloading:
		text, color = "DOWNLOADING", tcell.ColorBlue
	default:
		return tview.NewTableCell(" " + string(s))
	}
	return coloredCell(" "+text, color)
}

func progressTextForDL(r DownloadRecord) string {
	if r.TotalFiles == 0 {
		if r.Status == DLDownloading {
			return "fetching..."
		}
		return ""
	}
	pct := r.DoneFiles * 100 / r.TotalFiles
	base := fmt.Sprintf("%d%% (%d/%d)", pct, r.DoneFiles, r.TotalFiles)
	if r.FailedFiles > 0 && r.Status != DLDownloading {
		base += fmt.Sprintf(" [red]%d failed[-]", r.FailedFiles)
	}
	return base
}

func formatDLDate(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Format("02 Jan 15:04")
}

//nolint:unused // kept for future use in download path display
func truncateRight(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "..." + s[len(s)-max+3:]
}
