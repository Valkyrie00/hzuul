package views

import (
	"fmt"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SemaphoresView struct {
	root   *tview.Flex
	table  *tview.Table
	app    *tview.Application
	keyBar *KeyBar
	sems   []api.Semaphore
	filter string
}

func NewSemaphoresView(app *tview.Application, keyBar *KeyBar) *SemaphoresView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(NewSpacer(), 1, 0, false)
	root.SetBackgroundColor(ColorBg)

	return &SemaphoresView{root: root, table: table, app: app, keyBar: keyBar}
}

func (v *SemaphoresView) KeyHints() []KeyHint {
	return []KeyHint{HintFilter}
}

func (v *SemaphoresView) Root() tview.Primitive { return v.root }
func (v *SemaphoresView) UpdateStatus() {
	if n := v.table.GetRowCount() - 1; n > 0 {
		v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", n))
	} else {
		v.keyBar.ClearStatus()
	}
}
func (v *SemaphoresView) IsLiveFilterable() bool { return true }

func (v *SemaphoresView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *SemaphoresView) Load(client *api.Client) {
	firstLoad := len(v.sems) == 0
	sel, _ := v.table.GetSelection()

	go func() {
		sems, err := client.GetSemaphores()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Name", "Usage", "Current", "Max")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.sems = sems
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

func semaphoreBar(current, max int) string {
	if max <= 0 {
		return "[::d]—[-:-:-]"
	}
	const width = 12
	pct := float64(current) / float64(max)
	filled := int(pct*width + 0.5)
	if filled > width {
		filled = width
	}
	color := "[green]"
	if pct >= 0.8 {
		color = "[red]"
	} else if pct >= 0.5 {
		color = "[yellow]"
	}
	bar := color + strings.Repeat("█", filled) + "[-]" + "[::d]" + strings.Repeat("░", width-filled) + "[-:-:-]"
	return fmt.Sprintf("%s %s%3.0f%%[-]", bar, color, pct*100)
}

func (v *SemaphoresView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Usage", "Current", "Max")
	muted := ColorMuted
	row := 1
	for _, s := range v.sems {
		if !rowMatchesFilter(v.filter, s.Name) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+s.Name).SetTextColor(tcell.ColorWhite).SetExpansion(1))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+semaphoreBar(s.Value, s.Max)))
		v.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf(" %d", s.Value)).SetTextColor(muted))
		v.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf(" %d", s.Max)).SetTextColor(ColorAccent))
		row++
	}
	if row == 1 {
		msg := " [::d]No semaphores[-]"
		if v.filter != "" {
			msg = fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)
		}
		v.table.SetCell(1, 0, tview.NewTableCell(msg).SetExpansion(1))
	}
	v.keyBar.SetStatus(fmt.Sprintf("[::d]%d items[-:-:-]", row-1))
}
