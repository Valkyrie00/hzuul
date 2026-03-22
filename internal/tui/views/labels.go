package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type LabelsView struct {
	root  *tview.Flex
	table *tview.Table
	app   *tview.Application
}

func NewLabelsView(app *tview.Application) *LabelsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	return &LabelsView{root: root, table: table, app: app}
}

func (v *LabelsView) Root() tview.Primitive { return v.root }

func (v *LabelsView) Load(client *api.Client) {
	v.table.Clear()
	setTableHeader(v.table, "Label")

	go func() {
		labels, err := client.GetLabels()
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("[red]Error: %v[-]", err)))
				return
			}
			for i, l := range labels {
				v.table.SetCell(i+1, 0, tview.NewTableCell(l.Name).SetTextColor(tcell.ColorWhite))
			}
		})
	}()
}
