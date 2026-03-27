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
	client   *api.Client
	builds   []api.Build
	indexMap  []int
	filter   string
	curFilter api.BuildFilter
	skip     int
	loading  bool
	noMore   bool
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
		v.logView.ShowStaticLog(v.client, &build)
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

	table.SetSelectionChangedFunc(func(row, _ int) {
		dataRows := len(v.indexMap)
		if dataRows > 0 && row >= dataRows && !v.loading && !v.noMore {
			v.loadMore()
		}
	})

	logView.Root().(*tview.Flex).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.logView.Stop()
			v.pages.SwitchToPage("table")
			v.onDetail = false
			return nil
		}
		if event.Rune() == 'o' && v.logView.openURL != "" {
			openURL(v.logView.openURL)
			return nil
		}
		if event.Rune() == 'l' && v.logView.logURL != "" {
			openURL(v.logView.logURL)
			return nil
		}
		return event
	})

	return v
}

func (v *BuildsView) Root() tview.Primitive { return v.root }

func (v *BuildsView) SetFilter(term string) {
	v.filter = term
	if v.client == nil {
		return
	}
	v.curFilter = parseBuildFilter(term)
	v.skip = 0
	v.noMore = false
	v.searchServer()
}

func (v *BuildsView) buildIndex(row int) int {
	ri := row - 1
	if ri < 0 || ri >= len(v.indexMap) {
		return -1
	}
	return v.indexMap[ri]
}

func (v *BuildsView) Load(client *api.Client) {
	v.client = client
	if v.onDetail {
		return
	}
	if v.filter == "" {
		v.curFilter = api.BuildFilter{Limit: defaultPageSize}
	}
	v.skip = 0
	v.noMore = false
	v.searchServer()
}

func (v *BuildsView) searchServer() {
	if v.loading || v.client == nil {
		return
	}
	v.loading = true
	f := v.curFilter
	f.Skip = 0
	v.table.Clear()
	setBuildHeader(v.table)
	v.table.SetCell(1, 0, tview.NewTableCell(" [yellow]Searching...[-]").SetSelectable(false))

	go func() {
		builds, err := v.client.GetBuilds(&f)
		v.app.QueueUpdateDraw(func() {
			v.loading = false
			v.table.Clear()
			setBuildHeader(v.table)
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.builds = builds
			v.skip = len(builds)
			v.noMore = len(builds) < defaultPageSize
			v.renderRows(0)
			v.table.Select(1, 0)
			v.table.ScrollToBeginning()
		})
	}()
}

func (v *BuildsView) loadMore() {
	if v.loading || v.noMore || v.client == nil {
		return
	}
	v.loading = true
	f := v.curFilter
	f.Skip = v.skip

	lastRow := v.table.GetRowCount()
	v.table.SetCell(lastRow, 0, tview.NewTableCell(" [yellow]Loading more...[-]").SetSelectable(false))

	go func() {
		builds, err := v.client.GetBuilds(&f)
		v.app.QueueUpdateDraw(func() {
			v.loading = false
			v.table.RemoveRow(v.table.GetRowCount() - 1)
			if err != nil {
				return
			}
			if len(builds) == 0 {
				v.noMore = true
				return
			}
			startIdx := len(v.builds)
			v.builds = append(v.builds, builds...)
			v.skip += len(builds)
			v.noMore = len(builds) < defaultPageSize
			v.renderRows(startIdx)
		})
	}()
}

func setBuildHeader(table *tview.Table) {
	setTableHeader(table, "Job", "Project", "Branch", "Duration", "Result", "Pipeline", "Change", "Start")
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

func (v *BuildsView) renderRows(fromIdx int) {
	if fromIdx == 0 {
		v.indexMap = nil
	}
	muted := tcell.NewRGBColor(120, 120, 140)
	dim := tcell.NewRGBColor(90, 90, 110)
	row := fromIdx + 1
	for i := fromIdx; i < len(v.builds); i++ {
		b := v.builds[i]
		v.indexMap = append(v.indexMap, i)
		rc := resultColor(b.Result)
		v.table.SetCell(row, 0, tview.NewTableCell(" "+resultIcon(b.Result)+" "+b.JobName).SetTextColor(rc))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Project).SetTextColor(muted).SetMaxWidth(45))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(muted).SetMaxWidth(15))
		v.table.SetCell(row, 3, tview.NewTableCell(" "+formatBuildDuration(b.Duration)).SetTextColor(muted))
		v.table.SetCell(row, 4, resultCell(b.Result))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+truncate(b.Pipeline, 30)).SetTextColor(muted))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+formatChange(b.Ref)).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
		v.table.SetCell(row, 7, tview.NewTableCell(" "+b.StartTime).SetTextColor(dim).SetExpansion(1))
		row++
	}
	if len(v.builds) == 0 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No results for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
