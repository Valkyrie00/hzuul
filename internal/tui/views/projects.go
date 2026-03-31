package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
)

type ProjectsView struct {
	root       *tview.Flex
	table      *tview.Table
	buildTable *tview.Table
	logView    *BuildLogView
	pages      *tview.Pages
	app        *tview.Application
	client     *api.Client
	projects   []api.Project
	indexMap   []int
	filter     string

	buildBuilds  []api.Build
	buildProject string
	page         string // "table", "builds", "detail"
}

func NewProjectsView(app *tview.Application, dlManager *DownloadManager) *ProjectsView {
	bg := ColorBg
	navBg := ColorNavBg

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(bg)
	table.SetSelectedStyle(SelectedStyle)
	table.SetBorder(false)

	keys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(navBg)
	fmt.Fprint(keys, " [#3884f4]enter[-:-:-][::d]:recent builds[-:-:-]  [#3884f4]o[-:-:-][::d]:open in browser[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]")

	tableWithKeys := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	tableWithKeys.SetBackgroundColor(bg)

	buildTable := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	buildTable.SetBackgroundColor(bg)
	buildTable.SetSelectedStyle(SelectedStyle)

	buildHeader := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	buildHeader.SetBackgroundColor(navBg)

	buildKeys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	buildKeys.SetBackgroundColor(navBg)
	fmt.Fprint(buildKeys, " [#3884f4]enter[-:-:-][::d]:build detail[-:-:-]  [#3884f4]esc[-:-:-][::d]:back[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]")

	buildPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(buildHeader, 1, 0, false).
		AddItem(buildTable, 0, 1, true).
		AddItem(buildKeys, 1, 0, false)
	buildPage.SetBackgroundColor(bg)

	logView := NewBuildLogView(app, dlManager)

	pages := tview.NewPages().
		AddPage("table", tableWithKeys, true, true).
		AddPage("builds", buildPage, true, false).
		AddPage("detail", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(bg)

	v := &ProjectsView{
		root:       root,
		table:      table,
		buildTable: buildTable,
		logView:    logView,
		pages:      pages,
		app:        app,
		page:       "table",
	}

	table.SetSelectedFunc(func(row, _ int) {
		idx := v.projectIndex(row)
		if idx < 0 || v.client == nil {
			return
		}
		p := v.projects[idx]
		v.showProjectBuilds(p, buildHeader)
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'o' {
			row, _ := table.GetSelection()
			idx := v.projectIndex(row)
			if idx >= 0 && v.client != nil {
				p := v.projects[idx]
				openURL(v.client.ProjectURL(p.BestName()))
			}
			return nil
		}
		return event
	})

	buildTable.SetSelectedFunc(func(row, _ int) {
		bi := row - 1
		if bi < 0 || bi >= len(v.buildBuilds) {
			return
		}
		build := v.buildBuilds[bi]
		v.logView.ShowStaticLog(v.client, &build)
		v.pages.SwitchToPage("detail")
		v.page = "detail"
	})

	buildTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			v.pages.SwitchToPage("table")
			v.page = "table"
			return nil
		}
		return event
	})

	logView.SetBackHandler(func() {
		v.pages.SwitchToPage("builds")
		v.page = "builds"
	})

	return v
}

func (v *ProjectsView) Root() tview.Primitive { return v.root }

func (v *ProjectsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *ProjectsView) projectIndex(row int) int {
	ri := row - 1
	if ri < 0 || ri >= len(v.indexMap) {
		return -1
	}
	return v.indexMap[ri]
}

func (v *ProjectsView) Load(client *api.Client) {
	v.client = client
	if v.page != "table" {
		return
	}
	firstLoad := len(v.projects) == 0

	go func() {
		projects, err := client.GetProjects()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Name", "Type", "Connection")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.projects = projects
			v.renderTable()
			if firstLoad {
				v.table.Select(1, 0)
				v.table.ScrollToBeginning()
			}
		})
	}()
}

func (v *ProjectsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Type", "Connection")
	muted := ColorMuted
	v.indexMap = nil
	row := 1
	for i, p := range v.projects {
		if !rowMatchesFilter(v.filter, p.Name, p.Type, p.ConnectionName) {
			continue
		}
		v.indexMap = append(v.indexMap, i)
		v.table.SetCell(row, 0, tview.NewTableCell(" "+p.Name).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+p.Type).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+p.ConnectionName).SetTextColor(muted))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}

func (v *ProjectsView) showProjectBuilds(p api.Project, header *tview.TextView) {
	v.buildProject = p.Name
	v.buildTable.Clear()
	setTableHeader(v.buildTable, "Job", "Branch", "Pipeline", "Change", "Result", "Duration", "Start")
	v.buildTable.SetCell(1, 0, tview.NewTableCell(" [::d]Loading...[-:-:-]").SetSelectable(false))

	header.Clear()
	fmt.Fprintf(header, " [bold]%s[-:-:-]  [::d]recent builds[-:-:-]", p.Name)

	v.pages.SwitchToPage("builds")
	v.page = "builds"

	go func() {
		builds, err := v.client.GetBuilds(&api.BuildFilter{
			Project: p.Name,
			Limit:   30,
		})
		v.app.QueueUpdateDraw(func() {
			v.buildTable.Clear()
			setTableHeader(v.buildTable, "Job", "Branch", "Pipeline", "Change", "Result", "Duration", "Start")
			if err != nil {
				v.buildTable.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)).SetSelectable(false))
				return
			}
			if len(builds) == 0 {
				v.buildTable.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [yellow]No builds found for %s[-]", p.Name)).SetSelectable(false))
				return
			}
			v.buildBuilds = builds
			muted := ColorMuted
			dim := ColorDim
			for i, b := range builds {
				row := i + 1
				change := ""
				if b.Ref.Change != nil {
					change = fmt.Sprintf("%v", b.Ref.Change)
				}
				rc := resultColor(b.Result)
				v.buildTable.SetCell(row, 0, tview.NewTableCell(" "+resultIcon(b.Result)+" "+b.JobName).SetTextColor(rc))
				v.buildTable.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(muted))
				v.buildTable.SetCell(row, 2, tview.NewTableCell(" "+b.Pipeline).SetTextColor(muted))
				v.buildTable.SetCell(row, 3, tview.NewTableCell(" "+change).SetTextColor(muted))
				v.buildTable.SetCell(row, 4, resultCell(b.Result))
				v.buildTable.SetCell(row, 5, tview.NewTableCell(" "+formatBuildDuration(b.Duration)).SetTextColor(muted))
				v.buildTable.SetCell(row, 6, tview.NewTableCell(" "+formatTimestamp(b.StartTime)).SetTextColor(dim))
			}
			v.buildTable.Select(1, 0)
		})
	}()
}
