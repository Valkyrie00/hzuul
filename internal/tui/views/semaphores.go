package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type SemaphoresView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	sems   []api.Semaphore
	filter string
}

func NewSemaphoresView(app *tview.Application) *SemaphoresView {
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

	return &SemaphoresView{root: root, table: table, app: app}
}

func (v *SemaphoresView) Root() tview.Primitive { return v.root }

func (v *SemaphoresView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *SemaphoresView) Load(client *api.Client) {
	go func() {
		sems, err := client.GetSemaphores()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Name", "Max", "Current")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.sems = sems
			v.renderTable()
			v.table.Select(1, 0)
			v.table.ScrollToBeginning()
		})
	}()
}

func (v *SemaphoresView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Max", "Current")
	muted := tcell.NewRGBColor(120, 120, 140)
	row := 1
	for _, s := range v.sems {
		if !rowMatchesFilter(v.filter, s.Name) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+s.Name).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf(" %d", s.Max)).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
		v.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf(" %d", s.Value)).SetTextColor(muted))
		row++
	}
	if row == 1 {
		msg := " [::d]No semaphores[-]"
		if v.filter != "" {
			msg = fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)
		}
		v.table.SetCell(1, 0, tview.NewTableCell(msg).SetSelectable(false))
	}
}
