package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type ProjectsView struct {
	root     *tview.Flex
	table    *tview.Table
	app      *tview.Application
	projects []api.Project
	filter   string
}

func NewProjectsView(app *tview.Application) *ProjectsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))
	table.SetBorder(false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	return &ProjectsView{root: root, table: table, app: app}
}

func (v *ProjectsView) Root() tview.Primitive { return v.root }

func (v *ProjectsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
}

func (v *ProjectsView) Load(client *api.Client) {
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
		})
	}()
}

func (v *ProjectsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Type", "Connection")
	muted := tcell.NewRGBColor(120, 120, 140)
	row := 1
	for _, p := range v.projects {
		if !rowMatchesFilter(v.filter, p.Name, p.Type, p.ConnectionName) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+p.Name).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+p.Type).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+p.ConnectionName).SetTextColor(muted))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
