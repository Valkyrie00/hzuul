package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
)

type DownloadsView struct {
	root    *tview.Flex
	table   *tview.Table
	app     *tview.Application
	manager *DownloadManager
}

func NewDownloadsView(app *tview.Application, manager *DownloadManager) *DownloadsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	keys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(ColorNavBg)
	fmt.Fprint(keys, " [#3884f4]o[-:-:-][::d]:open dir[-:-:-]  [#3884f4]x[-:-:-][::d]:cancel[-:-:-]  [#3884f4]d[-:-:-][::d]:remove[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]")

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	root.SetBackgroundColor(ColorBg)

	v := &DownloadsView{
		root:    root,
		table:   table,
		app:     app,
		manager: manager,
	}

	manager.SetOnChange(func() {
		v.renderTable()
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		rec := v.selectedRecord()
		if rec == nil {
			return event
		}
		switch {
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

	return v
}

func (v *DownloadsView) Root() tview.Primitive { return v.root }

func (v *DownloadsView) Load(_ *api.Client) {
	v.renderTable()
}

func (v *DownloadsView) SetFilter(_ string) {}

func (v *DownloadsView) selectedRecord() *DownloadRecord {
	row, _ := v.table.GetSelection()
	idx := row - 1
	records := v.manager.Records()
	if idx < 0 || idx >= len(records) {
		return nil
	}
	return &records[idx]
}

func (v *DownloadsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Job", "Project", "Status", "Progress", "Size", "Date", "Path")

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

		v.table.SetCell(row, 0, tview.NewTableCell(" "+r.JobName).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+r.Project).SetTextColor(muted).SetMaxWidth(35))
		v.table.SetCell(row, 2, statusCell)
		v.table.SetCell(row, 3, tview.NewTableCell(" "+progressText).SetTextColor(muted))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+sizeText).SetTextColor(muted))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+dateText).SetTextColor(dim))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+truncateRight(r.DestDir, 40)).SetTextColor(dim))
	}
}

func statusCellForDL(s DLStatus) *tview.TableCell {
	switch s {
	case DLCompleted:
		return tview.NewTableCell(" [green]COMPLETED[-]")
	case DLFailed:
		return tview.NewTableCell(" [red]FAILED[-]")
	case DLCancelled:
		return tview.NewTableCell(" [yellow]CANCELLED[-]")
	case DLDownloading:
		return tview.NewTableCell(" [#3884f4]DOWNLOADING[-]")
	default:
		return tview.NewTableCell(" " + string(s))
	}
}

func progressTextForDL(r DownloadRecord) string {
	if r.TotalFiles == 0 {
		if r.Status == DLDownloading {
			return "fetching..."
		}
		return ""
	}
	pct := r.DoneFiles * 100 / r.TotalFiles
	return fmt.Sprintf("%d%% (%d/%d)", pct, r.DoneFiles, r.TotalFiles)
}

func formatDLDate(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Format("02 Jan 15:04")
}

func truncateRight(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "..." + s[len(s)-max+3:]
}
