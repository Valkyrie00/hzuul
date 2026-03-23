package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type BuildsView struct {
	root     *tview.Flex
	table    *tview.Table
	logView  *BuildLogView
	pages    *tview.Pages
	app      *tview.Application
	builds   []api.Build
	onDetail bool
}

func NewBuildsView(app *tview.Application) *BuildsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	logView := NewBuildLogView(app)

	pages := tview.NewPages().
		AddPage("table", table, true, true).
		AddPage("detail", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	v := &BuildsView{
		root:    root,
		table:   table,
		logView: logView,
		pages:   pages,
		app:     app,
	}

	table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1
		if idx < 0 || idx >= len(v.builds) {
			return
		}
		build := v.builds[idx]
		v.logView.ShowStaticLog(&build)
		v.pages.SwitchToPage("detail")
		v.onDetail = true
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'l' {
			row, _ := table.GetSelection()
			idx := row - 1
			if idx >= 0 && idx < len(v.builds) {
				build := v.builds[idx]
				v.logView.StreamBuild(nil, &build)
				v.pages.SwitchToPage("detail")
				v.onDetail = true
			}
			return nil
		}
		return event
	})

	logView.Root().(*tview.Flex).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.logView.Stop()
			v.pages.SwitchToPage("table")
			v.onDetail = false
			return nil
		}
		return event
	})

	return v
}

func (v *BuildsView) Root() tview.Primitive { return v.root }

func (v *BuildsView) Load(client *api.Client) {
	if v.onDetail {
		return
	}

	go func() {
		builds, err := client.GetBuilds(&api.BuildFilter{Limit: 50})
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Job", "Project", "Branch", "Pipeline", "Result", "Duration", "Start")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.builds = builds
			for i, b := range builds {
				row := i + 1
				v.table.SetCell(row, 0, tview.NewTableCell(" "+b.JobName).SetTextColor(tcell.ColorWhite))
				v.table.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Project).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 2, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 3, tview.NewTableCell(" ").SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 4, resultCell(b.Result))
				v.table.SetCell(row, 5, tview.NewTableCell(fmt.Sprintf(" %v", b.Duration)).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 6, tview.NewTableCell(" "+b.StartTime).SetTextColor(tcell.NewRGBColor(90, 90, 110)))
			}
		})
	}()
}
