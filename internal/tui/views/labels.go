package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type LabelsView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	labels []api.Label
	filter string
}

func NewLabelsView(app *tview.Application) *LabelsView {
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

	return &LabelsView{root: root, table: table, app: app}
}

func (v *LabelsView) Root() tview.Primitive { return v.root }

func (v *LabelsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
}

func (v *LabelsView) Load(client *api.Client) {
	go func() {
		labels, err := client.GetLabels()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Label")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.labels = labels
			v.renderTable()
		})
	}()
}

func (v *LabelsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Label")
	row := 1
	for _, l := range v.labels {
		if !rowMatchesFilter(v.filter, l.Name) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+l.Name).SetTextColor(tcell.ColorWhite))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
