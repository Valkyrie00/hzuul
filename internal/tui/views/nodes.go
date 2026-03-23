package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type NodesView struct {
	root  *tview.Flex
	table *tview.Table
	app   *tview.Application
}

func NewNodesView(app *tview.Application) *NodesView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	return &NodesView{root: root, table: table, app: app}
}

func (v *NodesView) Root() tview.Primitive { return v.root }

func (v *NodesView) Load(client *api.Client) {
	go func() {
		nodes, err := client.GetNodes()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "ID", "Label", "Provider", "State", "Connection")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			if len(nodes) == 0 {
				v.table.SetCell(1, 0, tview.NewTableCell(" [::d]No nodes[-]").SetSelectable(false))
				return
			}
			for i, n := range nodes {
				row := i + 1
				v.table.SetCell(row, 0, tview.NewTableCell(" "+n.ID).SetTextColor(tcell.ColorWhite))
				v.table.SetCell(row, 1, tview.NewTableCell(" "+n.Label).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 2, tview.NewTableCell(" "+n.Provider).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 3, stateCell(n.State))
				v.table.SetCell(row, 4, tview.NewTableCell(" "+n.Connection).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
			}
		})
	}()
}

func stateCell(state string) *tview.TableCell {
	var color tcell.Color
	switch state {
	case "ready":
		color = tcell.NewRGBColor(72, 199, 142)
	case "in-use":
		color = tcell.NewRGBColor(56, 132, 244)
	case "building", "deleting":
		color = tcell.NewRGBColor(242, 201, 76)
	default:
		color = tcell.NewRGBColor(120, 120, 140)
	}
	return tview.NewTableCell(" " + state).SetTextColor(color)
}
