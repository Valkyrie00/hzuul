package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type NodesView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	nodes  []api.Node
	filter string
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

func (v *NodesView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *NodesView) Load(client *api.Client) {
	go func() {
		nodes, err := client.GetNodes()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "ID", "Label", "Provider", "State", "Age", "Comment")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.nodes = nodes
			v.renderTable()
			v.table.Select(1, 0)
			v.table.ScrollToBeginning()
		})
	}()
}

func (v *NodesView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "ID", "Label", "Provider", "State", "Age", "Comment")
	muted := tcell.NewRGBColor(120, 120, 140)
	row := 1
	for _, n := range v.nodes {
		label := n.DisplayLabel()
		if !rowMatchesFilter(v.filter, n.ID, label, n.Provider, n.State, n.Comment) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+n.ID).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+label).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+n.Provider).SetTextColor(muted))
		v.table.SetCell(row, 3, stateCell(n.State))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+n.AgeString()).SetTextColor(muted))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+n.Comment).SetTextColor(muted).SetExpansion(1))
		row++
	}
	if row == 1 {
		msg := " [::d]No nodes[-]"
		if v.filter != "" {
			msg = fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)
		}
		v.table.SetCell(1, 0, tview.NewTableCell(msg).SetSelectable(false))
	}
}

func stateCell(state string) *tview.TableCell {
	var color tcell.Color
	switch state {
	case "ready":
		color = tcell.NewRGBColor(72, 199, 142)
	case "in-use":
		color = tcell.NewRGBColor(56, 132, 244)
	case "building":
		color = tcell.NewRGBColor(242, 201, 76)
	case "deleting":
		color = tcell.NewRGBColor(220, 60, 60)
	case "hold":
		color = tcell.NewRGBColor(200, 160, 80)
	default:
		color = tcell.NewRGBColor(120, 120, 140)
	}
	return tview.NewTableCell(" " + state).SetTextColor(color)
}
