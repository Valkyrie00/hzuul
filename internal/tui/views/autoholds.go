package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type AutoholdsView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	holds  []api.Autohold
	filter string
}

func NewAutoholdsView(app *tview.Application) *AutoholdsView {
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

	return &AutoholdsView{root: root, table: table, app: app}
}

func (v *AutoholdsView) Root() tview.Primitive { return v.root }

func (v *AutoholdsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *AutoholdsView) Load(client *api.Client) {
	go func() {
		holds, err := client.GetAutoholds()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "ID", "Project", "Job", "Count", "Reason")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.holds = holds
			v.renderTable()
		})
	}()
}

func (v *AutoholdsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "ID", "Project", "Job", "Count", "Reason")
	muted := tcell.NewRGBColor(120, 120, 140)
	row := 1
	for _, h := range v.holds {
		if !rowMatchesFilter(v.filter, h.ID, h.Project, h.Job, h.Reason) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+h.ID).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+h.Project).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+h.Job).SetTextColor(muted))
		v.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf(" %d/%d", h.CurrentCount, h.MaxCount)).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+h.Reason).SetTextColor(muted))
		row++
	}
	if row == 1 {
		msg := " [::d]No autoholds[-]"
		if v.filter != "" {
			msg = fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)
		}
		v.table.SetCell(1, 0, tview.NewTableCell(msg).SetSelectable(false))
	}
}
