package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type SemaphoresView struct {
	root  *tview.Flex
	table *tview.Table
	app   *tview.Application
}

func NewSemaphoresView(app *tview.Application) *SemaphoresView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	return &SemaphoresView{root: root, table: table, app: app}
}

func (v *SemaphoresView) Root() tview.Primitive { return v.root }

func (v *SemaphoresView) Load(client *api.Client) {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Max", "Current")

	go func() {
		sems, err := client.GetSemaphores()
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("[red]Error: %v[-]", err)))
				return
			}
			if len(sems) == 0 {
				v.table.SetCell(1, 0, tview.NewTableCell("[::d]No semaphores[-]").SetSelectable(false))
				return
			}
			for i, s := range sems {
				row := i + 1
				v.table.SetCell(row, 0, tview.NewTableCell(s.Name).SetTextColor(tcell.ColorWhite))
				v.table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", s.Max)).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
				v.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d", s.Value)).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
			}
		})
	}()
}
