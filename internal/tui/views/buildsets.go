package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type BuildsetsView struct {
	root  *tview.Flex
	table *tview.Table
	app   *tview.Application
}

func NewBuildsetsView(app *tview.Application) *BuildsetsView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)
	root.SetBackgroundColor(tcell.NewRGBColor(24, 24, 32))

	return &BuildsetsView{root: root, table: table, app: app}
}

func (v *BuildsetsView) Root() tview.Primitive { return v.root }

func (v *BuildsetsView) Load(client *api.Client) {
	v.table.Clear()
	setTableHeader(v.table, "Pipeline", "Project", "Change", "Result", "Start", "End")

	go func() {
		buildsets, err := client.GetBuildsets(&api.BuildFilter{Limit: 50})
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf("[red]Error: %v[-]", err)))
				return
			}
			for i, bs := range buildsets {
				row := i + 1
				v.table.SetCell(row, 0, tview.NewTableCell(bs.Pipeline).SetTextColor(tcell.ColorWhite))

				var projects []string
				for _, r := range bs.Refs {
					if r.Project != "" {
						projects = append(projects, r.Project)
					}
				}
				v.table.SetCell(row, 1, tview.NewTableCell(strings.Join(projects, ", ")).SetTextColor(tcell.NewRGBColor(120, 120, 140)))

				var change string
				if len(bs.Refs) > 0 && bs.Refs[0].Change != "" {
					change = fmt.Sprintf("%s,%s", bs.Refs[0].Change, bs.Refs[0].Patchset)
				}
				v.table.SetCell(row, 2, tview.NewTableCell(change).SetTextColor(tcell.NewRGBColor(120, 120, 140)))
				v.table.SetCell(row, 3, resultCell(bs.Result))
				v.table.SetCell(row, 4, tview.NewTableCell(bs.FirstBuildStart).SetTextColor(tcell.NewRGBColor(90, 90, 110)))
				v.table.SetCell(row, 5, tview.NewTableCell(bs.LastBuildEnd).SetTextColor(tcell.NewRGBColor(90, 90, 110)))
			}
		})
	}()
}
