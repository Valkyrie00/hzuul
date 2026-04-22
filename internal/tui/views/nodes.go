package views

import (
	"fmt"

	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type NodesView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	keyBar *KeyBar
	nodes  []api.Node
	filter string
}

func NewNodesView(app *tview.Application, keyBar *KeyBar) *NodesView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(NewSpacer(), 1, 0, false)
	root.SetBackgroundColor(ColorBg)

	return &NodesView{root: root, table: table, app: app, keyBar: keyBar}
}

func (v *NodesView) KeyHints() []KeyHint {
	return []KeyHint{HintFilter}
}

func (v *NodesView) Root() tview.Primitive { return v.root }
func (v *NodesView) UpdateStatus() {
	if n := v.table.GetRowCount() - 1; n > 0 {
		v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", n))
	} else {
		v.keyBar.ClearStatus()
	}
}
func (v *NodesView) IsLiveFilterable() bool { return true }

func (v *NodesView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *NodesView) Load(client *api.Client) {
	firstLoad := len(v.nodes) == 0
	sel, _ := v.table.GetSelection()

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

func (v *NodesView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "ID", "Label", "Provider", "State", "Age", "Comment")
	muted := ColorMuted
	row := 1
	for _, n := range v.nodes {
		label := n.DisplayLabel()
		if !rowMatchesFilter(v.filter, n.ID, label, n.Provider, n.State, n.Comment) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+n.ID).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+label).SetTextColor(muted).SetExpansion(1))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+n.Provider).SetTextColor(muted))
		v.table.SetCell(row, 3, stateCell(n.State))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+n.AgeString()).SetTextColor(muted))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+n.Comment).SetTextColor(muted))
		row++
	}
	if row == 1 {
		msg := " [::d]No nodes[-]"
		if v.filter != "" {
			msg = fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)
		}
		v.table.SetCell(1, 0, tview.NewTableCell(msg).SetExpansion(1))
	}
	v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", row-1))
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
	return coloredCell(" "+state, color)
}
