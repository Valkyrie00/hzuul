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
	indexMap  []int // visible table row index -> original builds slice index
	filter   string
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
		idx := v.buildIndex(row)
		if idx < 0 {
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
			idx := v.buildIndex(row)
			if idx >= 0 {
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

func (v *BuildsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
}

func (v *BuildsView) buildIndex(row int) int {
	ri := row - 1
	if ri < 0 || ri >= len(v.indexMap) {
		return -1
	}
	return v.indexMap[ri]
}

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
			v.renderTable()
		})
	}()
}

func (v *BuildsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Job", "Project", "Branch", "Pipeline", "Result", "Duration", "Start")
	muted := tcell.NewRGBColor(120, 120, 140)
	dim := tcell.NewRGBColor(90, 90, 110)
	v.indexMap = nil
	row := 1
	for i, b := range v.builds {
		if !rowMatchesFilter(v.filter, b.JobName, b.Ref.Project, b.Ref.Branch, b.Result) {
			continue
		}
		v.indexMap = append(v.indexMap, i)
		v.table.SetCell(row, 0, tview.NewTableCell(" "+b.JobName).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Project).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(muted))
		v.table.SetCell(row, 3, tview.NewTableCell(" ").SetTextColor(muted))
		v.table.SetCell(row, 4, resultCell(b.Result))
		v.table.SetCell(row, 5, tview.NewTableCell(fmt.Sprintf(" %v", b.Duration)).SetTextColor(muted))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+b.StartTime).SetTextColor(dim))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
