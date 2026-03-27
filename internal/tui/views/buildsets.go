package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type BuildsetsView struct {
	root         *tview.Flex
	table        *tview.Table
	pages        *tview.Pages
	detailFlex   *tview.Flex
	detailHead   *tview.TextView
	detailTbl    *tview.Table
	logView      *BuildLogView
	app          *tview.Application
	client       *api.Client
	buildsets    []api.Buildset
	detailBuilds []api.Build
	filter       string
	curFilter    api.BuildFilter
	skip         int
	loading      bool
	noMore       bool
	onDetail     bool
	onLog        bool
}

func NewBuildsetsView(app *tview.Application) *BuildsetsView {
	bg := tcell.NewRGBColor(24, 24, 32)

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(bg)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	detailHead := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	detailHead.SetBackgroundColor(bg)
	detailHead.SetBorderPadding(0, 0, 1, 0)

	detailTbl := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	detailTbl.SetBackgroundColor(bg)
	detailTbl.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	sep := tview.NewTextView().SetDynamicColors(true)
	sep.SetBackgroundColor(bg)
	sep.SetTextColor(tcell.NewRGBColor(50, 50, 65))
	fmt.Fprint(sep, "  ──────────────────────────────────────")

	detailKeys := tview.NewTextView().SetDynamicColors(true)
	detailKeys.SetBackgroundColor(bg)
	fmt.Fprint(detailKeys, " [blue]esc[-:-:-][::d]:back[-:-:-]  [blue]enter[-:-:-][::d]:build detail[-:-:-]  [blue]↑↓[-:-:-][::d]:navigate[-:-:-]")

	detailFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(detailHead, 0, 0, false).
		AddItem(sep, 1, 0, false).
		AddItem(detailTbl, 0, 1, true).
		AddItem(detailKeys, 1, 0, false)
	detailFlex.SetBackgroundColor(bg)

	logView := NewBuildLogView(app)

	pages := tview.NewPages().
		AddPage("table", table, true, true).
		AddPage("detail", detailFlex, true, false).
		AddPage("log", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(bg)

	v := &BuildsetsView{
		root:       root,
		table:      table,
		pages:      pages,
		detailFlex: detailFlex,
		detailHead: detailHead,
		detailTbl:  detailTbl,
		logView:    logView,
		app:        app,
	}

	table.SetSelectionChangedFunc(func(row, _ int) {
		dataRows := len(v.buildsets)
		if dataRows > 0 && row >= dataRows && !v.loading && !v.noMore {
			v.loadMore()
		}
	})

	table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1
		if idx < 0 || idx >= len(v.buildsets) {
			return
		}
		v.showDetail(v.buildsets[idx])
	})

	detailTbl.SetSelectedFunc(func(row, _ int) {
		idx := row - 1
		if idx < 0 || idx >= len(v.detailBuilds) {
			return
		}
		build := v.detailBuilds[idx]
		v.logView.ShowStaticLog(v.client, &build)
		v.pages.SwitchToPage("log")
		v.onLog = true
	})

	detailTbl.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.onDetail = false
			v.pages.SwitchToPage("table")
			v.app.SetFocus(v.table)
			return nil
		}
		return event
	})

	logView.Root().(*tview.Flex).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.logView.Stop()
			v.pages.SwitchToPage("detail")
			v.onLog = false
			v.app.SetFocus(v.detailTbl)
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

func (v *BuildsetsView) Root() tview.Primitive { return v.root }

func (v *BuildsetsView) SetFilter(term string) {
	v.filter = term
	if v.client == nil {
		return
	}
	v.curFilter = parseBuildsetFilter(term)
	v.skip = 0
	v.noMore = false
	v.searchServer()
}

func (v *BuildsetsView) Load(client *api.Client) {
	v.client = client
	if v.onDetail || v.onLog {
		return
	}
	if v.filter == "" {
		v.curFilter = api.BuildFilter{Limit: defaultPageSize}
	}
	v.skip = 0
	v.noMore = false
	v.searchServer()
}

func (v *BuildsetsView) searchServer() {
	if v.loading || v.client == nil {
		return
	}
	v.loading = true
	f := v.curFilter
	f.Skip = 0
	v.table.Clear()
	setBuildsetHeader(v.table)
	v.table.SetCell(1, 0, tview.NewTableCell(" [yellow]Searching...[-]").SetSelectable(false))

	go func() {
		buildsets, err := v.client.GetBuildsets(&f)
		v.app.QueueUpdateDraw(func() {
			v.loading = false
			v.table.Clear()
			setBuildsetHeader(v.table)
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.buildsets = buildsets
			v.skip = len(buildsets)
			v.noMore = len(buildsets) < defaultPageSize
			v.renderRows(0)
			v.table.Select(1, 0)
			v.table.ScrollToBeginning()
		})
	}()
}

func (v *BuildsetsView) loadMore() {
	if v.loading || v.noMore || v.client == nil {
		return
	}
	v.loading = true
	f := v.curFilter
	f.Skip = v.skip

	lastRow := v.table.GetRowCount()
	v.table.SetCell(lastRow, 0, tview.NewTableCell(" [yellow]Loading more...[-]").SetSelectable(false))

	go func() {
		buildsets, err := v.client.GetBuildsets(&f)
		v.app.QueueUpdateDraw(func() {
			v.loading = false
			v.table.RemoveRow(v.table.GetRowCount() - 1)
			if err != nil {
				return
			}
			if len(buildsets) == 0 {
				v.noMore = true
				return
			}
			startIdx := len(v.buildsets)
			v.buildsets = append(v.buildsets, buildsets...)
			v.skip += len(buildsets)
			v.noMore = len(buildsets) < defaultPageSize
			v.renderRows(startIdx)
		})
	}()
}

func (v *BuildsetsView) showDetail(bs api.Buildset) {
	v.onDetail = true
	v.detailHead.Clear()
	v.detailTbl.Clear()

	rc := resultColor(bs.Result)
	icon := resultIcon(bs.Result)

	projStr := buildsetProjects(bs)
	change := buildsetChange(bs)

	fmt.Fprintf(v.detailHead, " [::b]Buildset Result[-:-:-]\n")
	fmt.Fprintf(v.detailHead, "\n")
	fmt.Fprintf(v.detailHead, " [::b]%s[-:-:-] [%s]%s[-]\n", icon, colorHex(rc), bs.Result)
	fmt.Fprintf(v.detailHead, "\n")
	fmt.Fprintf(v.detailHead, " [::b]Project:[-:-:-]  %s\n", projStr)
	fmt.Fprintf(v.detailHead, " [::b]Pipeline:[-:-:-] %s\n", bs.Pipeline)
	if change != "" {
		fmt.Fprintf(v.detailHead, " [::b]Change:[-:-:-]   [#3884f4]%s[-]\n", change)
	}
	if len(bs.Refs) > 0 && bs.Refs[0].Branch != "" {
		fmt.Fprintf(v.detailHead, " [::b]Branch:[-:-:-]   %s\n", bs.Refs[0].Branch)
	}
	if len(bs.Refs) > 0 && bs.Refs[0].Ref != "" {
		fmt.Fprintf(v.detailHead, " [::b]Ref:[-:-:-]      %s\n", bs.Refs[0].Ref)
	}
	if bs.EventTimestamp != "" {
		fmt.Fprintf(v.detailHead, " [::b]Event:[-:-:-]    %s\n", bs.EventTimestamp)
	}
	if bs.FirstBuildStart != "" {
		fmt.Fprintf(v.detailHead, " [::b]Start:[-:-:-]    %s\n", bs.FirstBuildStart)
	}
	if bs.LastBuildEnd != "" {
		fmt.Fprintf(v.detailHead, " [::b]End:[-:-:-]      %s\n", bs.LastBuildEnd)
	}
	if bs.Message != "" {
		fmt.Fprintf(v.detailHead, " [::b]Message:[-:-:-]  %s\n", bs.Message)
	}

	lines := strings.Count(v.detailHead.GetText(true), "\n") + 1
	v.detailFlex.ResizeItem(v.detailHead, lines, 0)

	v.detailTbl.Clear()
	setTableHeader(v.detailTbl, "Job", "Result", "Duration", "Voting")

	v.pages.SwitchToPage("detail")

	go func() {
		full, err := v.client.GetBuildset(bs.UUID)
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.detailTbl.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error loading builds: %v[-]", err)))
				v.app.SetFocus(v.detailTbl)
				return
			}
			v.detailBuilds = full.Builds
			v.detailTbl.Clear()
			setTableHeader(v.detailTbl, "Job", "Result", "Duration", "Voting")

			muted := tcell.NewRGBColor(120, 120, 140)
			for i, b := range full.Builds {
				row := i + 1
				brc := resultColor(b.Result)
				voting := "yes"
				if bv, ok := b.Voting.(bool); ok && !bv {
					voting = "no"
				}
				v.detailTbl.SetCell(row, 0, tview.NewTableCell(" "+resultIcon(b.Result)+" "+b.JobName).SetTextColor(brc).SetExpansion(1))
				v.detailTbl.SetCell(row, 1, resultCell(b.Result))
				v.detailTbl.SetCell(row, 2, tview.NewTableCell(" "+formatBuildDuration(b.Duration)).SetTextColor(muted))
				v.detailTbl.SetCell(row, 3, tview.NewTableCell(" "+voting).SetTextColor(muted))
			}
			if len(full.Builds) == 0 {
				v.detailTbl.SetCell(1, 0, tview.NewTableCell(" [::d]No builds in this buildset[-]").SetSelectable(false))
			}
			v.detailTbl.Select(1, 0)
			v.app.SetFocus(v.detailTbl)
		})
	}()

	v.app.SetFocus(v.detailTbl)
}

func colorHex(c tcell.Color) string {
	r, g, b := c.RGB()
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func setBuildsetHeader(table *tview.Table) {
	setTableHeader(table, "Pipeline", "Result", "Project", "Change", "Start", "End")
}

func buildsetChange(bs api.Buildset) string {
	if len(bs.Refs) > 0 && bs.Refs[0].Change != nil {
		return fmt.Sprintf("%v,%v", bs.Refs[0].Change, bs.Refs[0].Patchset)
	}
	return ""
}

func buildsetProjects(bs api.Buildset) string {
	var projects []string
	for _, r := range bs.Refs {
		if r.Project != "" {
			projects = append(projects, r.Project)
		}
	}
	return strings.Join(projects, ", ")
}

func (v *BuildsetsView) renderRows(fromIdx int) {
	muted := tcell.NewRGBColor(120, 120, 140)
	dim := tcell.NewRGBColor(90, 90, 110)
	row := fromIdx + 1
	for i := fromIdx; i < len(v.buildsets); i++ {
		bs := v.buildsets[i]
		projStr := buildsetProjects(bs)
		change := buildsetChange(bs)

		rc := resultColor(bs.Result)
		v.table.SetCell(row, 0, tview.NewTableCell(" "+resultIcon(bs.Result)+" "+bs.Pipeline).SetTextColor(rc))
		v.table.SetCell(row, 1, resultCell(bs.Result))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+projStr).SetTextColor(muted).SetMaxWidth(50))
		v.table.SetCell(row, 3, tview.NewTableCell(" "+change).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+bs.FirstBuildStart).SetTextColor(dim))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+bs.LastBuildEnd).SetTextColor(dim).SetExpansion(1))
		row++
	}
	if len(v.buildsets) == 0 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No results for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
