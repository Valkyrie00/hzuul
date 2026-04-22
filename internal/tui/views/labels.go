package views

import (
	"fmt"

	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type LabelsView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	keyBar *KeyBar
	client *api.Client
	labels     []api.Label
	nodeCounts map[string]int
	filter     string
}

func NewLabelsView(app *tview.Application, keyBar *KeyBar) *LabelsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(NewSpacer(), 1, 0, false)
	root.SetBackgroundColor(ColorBg)

	return &LabelsView{root: root, table: table, app: app, keyBar: keyBar}
}

func (v *LabelsView) KeyHints() []KeyHint {
	return []KeyHint{HintFilter}
}

func (v *LabelsView) Root() tview.Primitive { return v.root }
func (v *LabelsView) UpdateStatus() {
	if n := v.table.GetRowCount() - 1; n > 0 {
		v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", n))
	} else {
		v.keyBar.ClearStatus()
	}
}
func (v *LabelsView) IsLiveFilterable() bool { return true }

func (v *LabelsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *LabelsView) Load(client *api.Client) {
	v.client = client
	firstLoad := len(v.labels) == 0
	sel, _ := v.table.GetSelection()

	go func() {
		labels, err := client.GetLabels()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Label", "Nodes")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.labels = labels
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

		nodes, err := client.GetNodes()
		if err != nil {
			return
		}
		counts := make(map[string]int)
		for _, n := range nodes {
			lbl := n.DisplayLabel()
			if lbl != "" {
				counts[lbl]++
			}
		}
		v.app.QueueUpdateDraw(func() {
			v.nodeCounts = counts
			v.renderTable()
		})
	}()
}

func (v *LabelsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Label", "Nodes")
	row := 1
	for _, l := range v.labels {
		if !rowMatchesFilter(v.filter, l.Name) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+l.Name).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		if v.nodeCounts != nil {
			cnt := v.nodeCounts[l.Name]
			if cnt > 0 {
				v.table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf(" %d", cnt)).SetTextColor(ColorAccent))
			} else {
				v.table.SetCell(row, 1, tview.NewTableCell(" —").SetTextColor(ColorMuted))
			}
		} else {
			v.table.SetCell(row, 1, tview.NewTableCell("").SetTextColor(ColorMuted))
		}
		row++
	}
	if row == 1 {
		msg := " [::d]No labels[-]"
		if v.filter != "" {
			msg = fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)
		}
		v.table.SetCell(1, 0, tview.NewTableCell(msg).SetExpansion(1))
	}
	v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", row-1))
}
