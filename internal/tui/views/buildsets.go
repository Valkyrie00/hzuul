package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type BuildsetsView struct {
	root      *tview.Flex
	table     *tview.Table
	app       *tview.Application
	buildsets []api.Buildset
	filter    string
}

func NewBuildsetsView(app *tview.Application) *BuildsetsView {
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

	return &BuildsetsView{root: root, table: table, app: app}
}

func (v *BuildsetsView) Root() tview.Primitive { return v.root }

func (v *BuildsetsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
}

func (v *BuildsetsView) Load(client *api.Client) {
	go func() {
		buildsets, err := client.GetBuildsets(&api.BuildFilter{Limit: 50})
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Pipeline", "Project", "Change", "Result", "Start", "End")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.buildsets = buildsets
			v.renderTable()
		})
	}()
}

func (v *BuildsetsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Pipeline", "Project", "Change", "Result", "Start", "End")
	muted := tcell.NewRGBColor(120, 120, 140)
	dim := tcell.NewRGBColor(90, 90, 110)
	row := 1
	for _, bs := range v.buildsets {
		var projects []string
		for _, r := range bs.Refs {
			if r.Project != "" {
				projects = append(projects, r.Project)
			}
		}
		projStr := strings.Join(projects, ", ")

		var change string
		if len(bs.Refs) > 0 && bs.Refs[0].Change != nil {
			change = fmt.Sprintf("%v,%v", bs.Refs[0].Change, bs.Refs[0].Patchset)
		}

		if !rowMatchesFilter(v.filter, bs.Pipeline, projStr, change, bs.Result) {
			continue
		}

		v.table.SetCell(row, 0, tview.NewTableCell(" "+bs.Pipeline).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+projStr).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+change).SetTextColor(muted))
		v.table.SetCell(row, 3, resultCell(bs.Result))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+bs.FirstBuildStart).SetTextColor(dim))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+bs.LastBuildEnd).SetTextColor(dim))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}
