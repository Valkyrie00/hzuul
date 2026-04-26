package views

import (
	"fmt"

	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ProjectsView struct {
	root       *tview.Flex
	table      *tview.Table
	buildTable *tview.Table
	logView    *BuildLogView
	pages      *tview.Pages
	keyBar     *KeyBar
	app        *tview.Application
	client     *api.Client
	projects   []api.Project
	indexMap   []int
	filter     string

	buildBuilds  []api.Build
	buildProject string
	page         string // "table", "builds", "detail"
}

func NewProjectsView(app *tview.Application, keyBar *KeyBar, dlManager *DownloadManager, aiCfg config.AIConfig) *ProjectsView {
	bg := ColorBg
	navBg := ColorNavBg

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(bg)
	table.SetSelectedStyle(SelectedStyle)
	table.SetBorder(false)

	tablePage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(NewSpacer(), 1, 0, false)
	tablePage.SetBackgroundColor(bg)

	buildTable := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	buildTable.SetBackgroundColor(bg)
	buildTable.SetSelectedStyle(SelectedStyle)

	buildHeader := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	buildHeader.SetBackgroundColor(navBg)

	buildPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(buildHeader, 1, 0, false).
		AddItem(buildTable, 0, 1, true).
		AddItem(NewSpacer(), 1, 0, false)
	buildPage.SetBackgroundColor(bg)

	logView := NewBuildLogView(app, keyBar, dlManager, aiCfg)

	pages := tview.NewPages().
		AddPage("table", tablePage, true, true).
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
		keyBar:     keyBar,
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
		if build.Result == "" && build.UUID != "" {
			v.logView.StreamBuild(v.client, &build)
		} else {
			v.logView.ShowStaticLog(v.client, &build)
		}
		v.pages.SwitchToPage("detail")
		v.page = "detail"
	})

	buildTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			v.pages.SwitchToPage("table")
			v.page = "table"
			v.keyBar.SetHints(v.KeyHints())
			v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", v.table.GetRowCount()-1))
			return nil
		}
		bi := func() int {
			r, _ := buildTable.GetSelection()
			return r - 1
		}()
		if bi < 0 || bi >= len(v.buildBuilds) {
			return event
		}
		build := v.buildBuilds[bi]
		return handleBuildOpenKeys(event, v.client, &build)
	})

	logView.SetBackHandler(func() {
		v.pages.SwitchToPage("builds")
		v.page = "builds"
		v.keyBar.SetHints(v.KeyHints())
	})

	return v
}

func (v *ProjectsView) SetBookmarkManager(bm *BookmarkManager) { v.logView.SetBookmarkManager(bm) }
func (v *ProjectsView) Root() tview.Primitive { return v.root }
func (v *ProjectsView) UpdateStatus() {
	if v.page == "table" {
		v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", v.table.GetRowCount()-1))
	} else {
		v.keyBar.ClearStatus()
	}
}

func (v *ProjectsView) KeyHints() []KeyHint {
	switch v.page {
	case "builds":
		return []KeyHint{HintEnter, HintOpenWeb, HintOpenChange, HintBack}
	case "detail":
		return v.logView.KeyHints()
	default:
		return []KeyHint{HintRecent, HintOpenBrowser, HintFilter}
	}
}

func (v *ProjectsView) IsModal() bool          { return v.logView.IsAnalysisActive() || v.logView.IsInputActive() }
func (v *ProjectsView) CanReconnect() bool     { return v.logView.CanReconnect() }
func (v *ProjectsView) Reconnect()             { v.logView.Reconnect() }
func (v *ProjectsView) IsLiveFilterable() bool { return true }

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
	sel, _ := v.table.GetSelection()

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
			} else {
				if sel >= v.table.GetRowCount() {
					sel = v.table.GetRowCount() - 1
				}
				if sel < 1 {
					sel = 1
				}
				v.table.Select(sel, 0)
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
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetExpansion(1))
	}
	v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", row-1))
}

func (v *ProjectsView) showProjectBuilds(p api.Project, header *tview.TextView) {
	v.buildProject = p.Name
	v.buildTable.Clear()
	setTableHeader(v.buildTable, "Job", "Branch", "Pipeline", "Change", "Result", "Duration", "Start")
	v.buildTable.SetCell(1, 0, tview.NewTableCell(" [::d]Loading...[-:-:-]").SetExpansion(1))

	header.Clear()
	_, _ = fmt.Fprintf(header, " [bold]%s[-:-:-]  [::d]recent builds[-:-:-]", p.Name)

	v.keyBar.ClearStatus()
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
				v.buildTable.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)).SetExpansion(1))
				return
			}
			if len(builds) == 0 {
				v.buildTable.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [yellow]No builds found for %s[-]", p.Name)).SetExpansion(1))
				return
			}
			v.buildBuilds = builds
			renderBuildRows(v.buildTable, builds, func(b api.Build) string { return b.JobName })
		})
	}()
}
