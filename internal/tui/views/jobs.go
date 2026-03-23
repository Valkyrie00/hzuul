package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type JobsView struct {
	root  *tview.Flex
	table *tview.Table
	app   *tview.Application
}

func NewJobsView(app *tview.Application) *JobsView {
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

	return &JobsView{root: root, table: table, app: app}
}

func (v *JobsView) Root() tview.Primitive { return v.root }

func (v *JobsView) Load(client *api.Client) {
	go func() {
		jobs, err := client.GetJobs()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Name", "Description", "Tags")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			for i, j := range jobs {
				row := i + 1
				v.table.SetCell(row, 0, tview.NewTableCell(" "+j.Name).SetTextColor(tcell.ColorWhite))
				desc := j.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				v.table.SetCell(row, 1, tview.NewTableCell(" "+desc).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 2, tview.NewTableCell(" "+strings.Join(j.Tags, ", ")).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
			}
		})
	}()
}
