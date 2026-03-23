package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type JobsView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	jobs   []api.Job
	filter string
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

func (v *JobsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
}

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
			v.jobs = jobs
			v.renderTable()
		})
	}()
}

func (v *JobsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Description", "Tags")
	muted := tcell.NewRGBColor(120, 120, 140)
	row := 1
	for _, j := range v.jobs {
		tags := strings.Join(j.Tags, ", ")
		if !rowMatchesFilter(v.filter, j.Name, j.Description, tags) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+j.Name).SetTextColor(tcell.ColorWhite))
		desc := j.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		v.table.SetCell(row, 1, tview.NewTableCell(" "+desc).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+tags).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
