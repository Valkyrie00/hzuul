package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type ProjectsView struct {
	root  *tview.Flex
	table *tview.Table
	app   *tview.Application
}

func NewProjectsView(app *tview.Application) *ProjectsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))
	table.SetBorder(false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	return &ProjectsView{root: root, table: table, app: app}
}

func (v *ProjectsView) Root() tview.Primitive { return v.root }

func (v *ProjectsView) Load(client *api.Client) {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Type", "Connection")

	go func() {
		projects, err := client.GetProjects()
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("[red]Error: %v[-]", err)))
				return
			}
			for i, p := range projects {
				row := i + 1
				v.table.SetCell(row, 0, tview.NewTableCell(p.Name).SetTextColor(tcell.ColorWhite))
				v.table.SetCell(row, 1, tview.NewTableCell(p.Type).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 2, tview.NewTableCell(p.ConnectionName).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
			}
		})
	}()
}
