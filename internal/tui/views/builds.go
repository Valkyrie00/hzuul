package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
)

type BuildsView struct {
	root       *tview.Flex
	table      *tview.Table
	keys       *tview.TextView
	countLabel *tview.TextView
	dlLabel    *tview.TextView
	logView    *BuildLogView
	dlManager  *DownloadManager
	pages      *tview.Pages
	app        *tview.Application
	client     *api.Client
	builds     []api.Build
	indexMap    []int
	filter     string
	curFilter  api.BuildFilter
	skip       int
	loading    bool
	noMore     bool
	onDetail   bool
	dequeuePending  bool
	dequeueBuildIdx int
}

func NewBuildsView(app *tview.Application, dlManager *DownloadManager, aiCfg config.AIConfig) *BuildsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	countLabel := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight)
	countLabel.SetBackgroundColor(ColorBg)

	dlLabel := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight)
	dlLabel.SetBackgroundColor(ColorNavBg)

	keys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(ColorNavBg)

	keysRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(keys, 0, 1, false).
		AddItem(dlLabel, 22, 0, false).
		AddItem(countLabel, 20, 0, false)
	keysRow.SetBackgroundColor(ColorNavBg)

	tableWithKeys := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keysRow, 1, 0, false)
	tableWithKeys.SetBackgroundColor(ColorBg)

	logView := NewBuildLogView(app, dlManager, aiCfg)

	pages := tview.NewPages().
		AddPage("table", tableWithKeys, true, true).
		AddPage("detail", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(ColorBg)

	v := &BuildsView{
		root:       root,
		table:      table,
		keys:       keys,
		countLabel: countLabel,
		dlLabel:    dlLabel,
		logView:    logView,
		dlManager:  dlManager,
		pages:      pages,
		app:        app,
	}

	dlManager.SetOnChange(func() {
		v.updateDLLabel()
	})

	table.SetSelectedFunc(func(row, _ int) {
		if v.loading {
			return
		}
		idx := v.buildIndex(row)
		if idx < 0 || v.client == nil {
			return
		}
		build := v.builds[idx]
		if build.Result == "" && build.UUID != "" {
			v.logView.StreamBuild(v.client, &build)
		} else {
			v.logView.ShowStaticLog(v.client, &build)
		}
		v.pages.SwitchToPage("detail")
		v.onDetail = true
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if v.dequeuePending {
			switch event.Rune() {
			case 'y', 'Y':
				v.executeDequeue()
			case 'n', 'N':
				v.cancelDequeue()
			}
			return nil
		}
		if v.loading {
			return event
		}
		row, _ := table.GetSelection()
		idx := v.buildIndex(row)
		if idx < 0 || v.client == nil {
			return event
		}
		build := v.builds[idx]
		if event.Rune() == 'x' {
			v.confirmDequeue(idx)
			return nil
		}
		return handleBuildOpenKeys(event, v.client, &build)
	})

	table.SetSelectionChangedFunc(func(row, _ int) {
		dataRows := len(v.indexMap)
		if dataRows > 0 && row >= dataRows && !v.loading && !v.noMore {
			v.loadMore()
		}
	})

	logView.SetBackHandler(func() {
		v.pages.SwitchToPage("table")
		v.onDetail = false
	})

	return v
}

func (v *BuildsView) updateDLLabel() {
	v.dlLabel.Clear()
	for _, r := range v.dlManager.Records() {
		if r.Status == DLDownloading {
			pct := 0
			if r.TotalFiles > 0 {
				pct = r.DoneFiles * 100 / r.TotalFiles
			}
			fmt.Fprintf(v.dlLabel, "[yellow::b]↓[-:-:-][::d] %d%% (%d/%d)[-:-:-] ", pct, r.DoneFiles, r.TotalFiles)
			return
		}
	}
}

func (v *BuildsView) SetBookmarkManager(bm *BookmarkManager) { v.logView.SetBookmarkManager(bm) }
func (v *BuildsView) Root() tview.Primitive                   { return v.root }

func (v *BuildsView) updateBuildsKeys() {
	v.keys.Clear()
	if v.dequeuePending {
		b := v.builds[v.dequeueBuildIdx]
		fmt.Fprintf(v.keys, " [red::b]Dequeue[-:-:-] [white]%s[-] [::d]from %s[-:-:-]  [#48c78e::b]y[-:-:-][::d]:confirm[-:-:-]  [#eb5757::b]n[-:-:-][::d]:cancel[-:-:-]",
			truncate(b.JobName, 30), b.Pipeline)
		return
	}
	base := " [#3884f4]enter[-:-:-][::d]:build detail[-:-:-]  [#3884f4]o[-:-:-][::d]:open web[-:-:-]  [#3884f4]c[-:-:-][::d]:open change[-:-:-]"
	if v.client != nil && v.client.HasAdminToken() {
		base += "  [#3884f4]x[-:-:-][::d]:dequeue[-:-:-]"
	}
	base += "  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]"
	fmt.Fprint(v.keys, base)
}

func (v *BuildsView) confirmDequeue(buildIdx int) {
	if v.client == nil || !v.client.HasAdminToken() {
		return
	}
	b := v.builds[buildIdx]
	if b.Result != "" {
		return
	}
	v.dequeuePending = true
	v.dequeueBuildIdx = buildIdx
	v.updateBuildsKeys()
}

func (v *BuildsView) cancelDequeue() {
	v.dequeuePending = false
	v.updateBuildsKeys()
}

func (v *BuildsView) executeDequeue() {
	b := v.builds[v.dequeueBuildIdx]
	v.keys.Clear()
	fmt.Fprint(v.keys, " [yellow::b]Dequeuing...[-:-:-]")

	go func() {
		req := &api.DequeueRequest{
			Pipeline: b.Pipeline,
			Project:  b.Ref.Project,
		}
		if b.Ref.Change != nil && b.Ref.Patchset != nil {
			req.Change = fmt.Sprintf("%v,%v", b.Ref.Change, b.Ref.Patchset)
		} else if b.Ref.Ref != "" {
			req.Ref = b.Ref.Ref
		}
		err := v.client.Dequeue(b.Ref.Project, req)
		v.app.QueueUpdateDraw(func() {
			v.dequeuePending = false
			if err != nil {
				v.keys.Clear()
				fmt.Fprintf(v.keys, " [red]Error: %v[-]", err)
				return
			}
			v.updateBuildsKeys()
			v.Load(v.client)
		})
	}()
}

func (v *BuildsView) IsModal() bool { return v.logView.IsAnalysisActive() }

func (v *BuildsView) SetFilter(term string) {
	v.filter = term
	if v.client == nil {
		return
	}
	v.curFilter = parseBuildFilter(term)
	v.skip = 0
	v.noMore = false
	v.countLabel.Clear()
	fmt.Fprint(v.countLabel, "[yellow::b]Searching...[-:-:-] ")
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
	v.updateBuildsKeys()
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
	firstLoad := len(v.builds) == 0

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
			v.updateCount()
			if firstLoad {
				v.table.Select(1, 0)
				v.table.ScrollToBeginning()
			}
		})
	}()
}

func (v *BuildsView) updateCount() {
	v.countLabel.Clear()
	suffix := ""
	if !v.noMore {
		suffix = "+"
	}
	fmt.Fprintf(v.countLabel, "[::d]%d%s items [-:-:-]", len(v.builds), suffix)
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
			v.updateCount()
		})
	}()
}

func setBuildHeader(table *tview.Table) {
	setTableHeader(table, "Job", "Project", "Branch", "Duration", "Result", "Pipeline", "Change", "Start")
}

func (v *BuildsView) renderRows(fromIdx int) {
	if fromIdx == 0 {
		v.indexMap = nil
	}
	muted := ColorMuted
	dim := ColorDim
	row := fromIdx + 1
	for i := fromIdx; i < len(v.builds); i++ {
		b := v.builds[i]
		v.indexMap = append(v.indexMap, i)
		rc := resultColor(b.Result)
		v.table.SetCell(row, 0, coloredCell(" "+resultIcon(b.Result)+" "+b.JobName, rc).SetExpansion(1))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Project).SetTextColor(muted).SetMaxWidth(45))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(muted).SetMaxWidth(15))
		v.table.SetCell(row, 3, tview.NewTableCell(" "+formatBuildDuration(b.Duration)).SetTextColor(muted))
		v.table.SetCell(row, 4, resultCell(b.Result))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+truncate(b.Pipeline, 30)).SetTextColor(muted))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+formatChange(b.Ref)).SetTextColor(ColorAccent))
		v.table.SetCell(row, 7, tview.NewTableCell(" "+formatTimestamp(b.StartTime)).SetTextColor(dim))
		row++
	}
	if len(v.builds) == 0 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No results for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
