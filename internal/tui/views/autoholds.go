package views

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type AutoholdsView struct {
	root   *tview.Flex
	table  *tview.Table
	pages  *tview.Pages
	keys   *tview.TextView
	app    *tview.Application
	client *api.Client
	holds  []api.Autohold
	filter string
	modal  bool
}

func (v *AutoholdsView) IsModal() bool { return v.modal }

func NewAutoholdsView(app *tview.Application) *AutoholdsView {
	bg := tcell.NewRGBColor(24, 24, 32)

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(bg)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	keys := tview.NewTextView().SetDynamicColors(true)
	keys.SetBackgroundColor(bg)
	fmt.Fprint(keys, " [blue]c[-:-:-][::d]:create[-:-:-]  [blue]d[-:-:-][::d]:delete[-:-:-]  [blue]enter[-:-:-][::d]:detail[-:-:-]  [blue]↑↓[-:-:-][::d]:navigate[-:-:-]")

	tablePage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	tablePage.SetBackgroundColor(bg)

	pages := tview.NewPages().
		AddPage("table", tablePage, true, true)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(bg)

	v := &AutoholdsView{
		root:  root,
		table: table,
		pages: pages,
		keys:  keys,
		app:   app,
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'c':
			v.showCreateForm()
			return nil
		case 'd':
			v.confirmDelete()
			return nil
		}
		return event
	})

	return v
}

func (v *AutoholdsView) Root() tview.Primitive { return v.root }

func (v *AutoholdsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *AutoholdsView) Load(client *api.Client) {
	v.client = client
	go func() {
		holds, err := client.GetAutoholds()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setAutoholdHeader(v.table)
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.holds = holds
			v.renderTable()
			v.table.Select(1, 0)
			v.table.ScrollToBeginning()
		})
	}()
}

func setAutoholdHeader(table *tview.Table) {
	setTableHeader(table, "ID", "Project", "Job", "Ref Filter", "Count", "Hold for", "Reason")
}

func (v *AutoholdsView) renderTable() {
	v.table.Clear()
	setAutoholdHeader(v.table)
	muted := tcell.NewRGBColor(120, 120, 140)
	row := 1
	for _, h := range v.holds {
		if !rowMatchesFilter(v.filter, h.ID, h.Project, h.Job, h.RefFilter, h.Reason) {
			continue
		}
		v.table.SetCell(row, 0, tview.NewTableCell(" "+h.ID).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+h.Project).SetTextColor(muted))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+h.Job).SetTextColor(muted))
		v.table.SetCell(row, 3, tview.NewTableCell(" "+h.RefFilter).SetTextColor(muted))
		v.table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf(" %d/%d", h.CurrentCount, h.MaxCount)).SetTextColor(tcell.NewRGBColor(56, 132, 244)))
		v.table.SetCell(row, 5, tview.NewTableCell(" "+h.HoldDuration()).SetTextColor(muted))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+h.Reason).SetTextColor(muted).SetExpansion(1))
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

func (v *AutoholdsView) closeForm() {
	v.modal = false
	v.pages.SwitchToPage("table")
	v.pages.RemovePage("create")
	v.app.SetFocus(v.table)
}

func (v *AutoholdsView) showCreateForm() {
	bg := tcell.NewRGBColor(24, 24, 32)
	fieldBg := tcell.NewRGBColor(40, 40, 55)
	labelColor := tcell.NewRGBColor(56, 132, 244)
	btnBg := tcell.NewRGBColor(56, 132, 244)
	btnActiveBg := tcell.NewRGBColor(72, 199, 142)
	cancelBg := tcell.NewRGBColor(200, 50, 50)

	form := tview.NewForm()
	form.SetBackgroundColor(bg)
	form.SetFieldBackgroundColor(fieldBg)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(labelColor)
	form.SetButtonBackgroundColor(btnBg)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetButtonActivatedStyle(tcell.StyleDefault.Background(btnActiveBg).Foreground(tcell.ColorBlack))

	numAccept := func(text string, ch rune) bool {
		_, err := strconv.Atoi(text + string(ch))
		return err == nil || text+string(ch) == ""
	}

	form.AddInputField("Project *", "", 0, nil, nil)
	form.AddInputField("Job *", "", 0, nil, nil)
	form.AddInputField("Change", "", 0, nil, nil)
	form.AddInputField("Ref", "", 0, nil, nil)
	form.AddInputField("Reason *", "Requested from HZUUL", 0, nil, nil)
	form.AddInputField("Count", "1", 0, numAccept, nil)
	form.AddInputField("Node Hold Expires in (s)", "86400", 0, numAccept, nil)

	statusBar := tview.NewTextView().SetDynamicColors(true)
	statusBar.SetBackgroundColor(bg)

	createStyle := tcell.StyleDefault.Background(btnActiveBg).Foreground(tcell.ColorBlack)
	cancelActiveStyle := tcell.StyleDefault.Background(tcell.NewRGBColor(235, 87, 87)).Foreground(tcell.ColorWhite)

	form.AddButton("Create", func() {
		project := form.GetFormItemByLabel("Project *").(*tview.InputField).GetText()
		job := form.GetFormItemByLabel("Job *").(*tview.InputField).GetText()
		change := form.GetFormItemByLabel("Change").(*tview.InputField).GetText()
		ref := form.GetFormItemByLabel("Ref").(*tview.InputField).GetText()
		reason := form.GetFormItemByLabel("Reason *").(*tview.InputField).GetText()
		countStr := form.GetFormItemByLabel("Count").(*tview.InputField).GetText()
		expiryStr := form.GetFormItemByLabel("Node Hold Expires in (s)").(*tview.InputField).GetText()

		if project == "" || job == "" || reason == "" {
			statusBar.Clear()
			fmt.Fprint(statusBar, " [red]Project, Job and Reason are required[-]")
			return
		}

		count, _ := strconv.Atoi(countStr)
		if count <= 0 {
			count = 1
		}
		expiry, _ := strconv.Atoi(expiryStr)
		if expiry <= 0 {
			expiry = 86400
		}

		var changePtr, refPtr *string
		if change != "" {
			changePtr = &change
		}
		if ref != "" {
			refPtr = &ref
		}

		req := &api.AutoholdRequest{
			Job:            job,
			Change:         changePtr,
			Ref:            refPtr,
			Reason:         reason,
			Count:          count,
			NodeHoldExpiry: expiry,
		}

		statusBar.Clear()
		fmt.Fprint(statusBar, " [yellow]Creating autohold request...[-]")

		go func() {
			err := v.client.CreateAutohold(project, req)
			v.app.QueueUpdateDraw(func() {
				if err != nil {
					statusBar.Clear()
					fmt.Fprintf(statusBar, " [red]Error: %v[-]", err)
					return
				}
				v.closeForm()
				v.Load(v.client)
			})
		}()
	})

	form.AddButton("Cancel", func() {
		v.closeForm()
	})
	cancelBtn := form.GetButton(form.GetButtonCount() - 1)
	cancelBtn.SetBackgroundColor(cancelBg)

	form.SetCancelFunc(func() {
		v.closeForm()
	})

	totalItems := form.GetFormItemCount()
	focusIdx := 0
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			focusIdx = (focusIdx + 1) % (totalItems + 2)
		case tcell.KeyBacktab:
			focusIdx = (focusIdx - 1 + totalItems + 2) % (totalItems + 2)
		case tcell.KeyEnter:
			if focusIdx < totalItems {
				focusIdx = (focusIdx + 1) % (totalItems + 2)
			}
		}
		if focusIdx == totalItems+1 {
			form.SetButtonActivatedStyle(cancelActiveStyle)
		} else {
			form.SetButtonActivatedStyle(createStyle)
		}
		return event
	})

	header := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	header.SetBackgroundColor(bg)
	fmt.Fprint(header, " [bold]Create Autohold Request[-]")

	hint := tview.NewTextView().SetDynamicColors(true)
	hint.SetBackgroundColor(bg)
	fmt.Fprint(hint, " [blue]tab[-:-:-][::d]:next field[-:-:-]  [blue]shift+tab[-:-:-][::d]:prev field[-:-:-]  [blue]enter[-:-:-][::d]:confirm[-:-:-]  [blue]esc[-:-:-][::d]:cancel[-:-:-]")

	formPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(form, 0, 1, true).
		AddItem(statusBar, 1, 0, false).
		AddItem(hint, 1, 0, false)
	formPage.SetBackgroundColor(bg)

	v.modal = true
	v.pages.AddAndSwitchToPage("create", formPage, true)
	v.app.SetFocus(form)
}

func (v *AutoholdsView) closeConfirm() {
	v.modal = false
	v.pages.SwitchToPage("table")
	v.pages.RemovePage("confirm")
	v.app.SetFocus(v.table)
}

func (v *AutoholdsView) confirmDelete() {
	row, _ := v.table.GetSelection()
	if row < 1 || row > len(v.holds) {
		return
	}

	idx := row - 1
	filtered := 0
	for i, h := range v.holds {
		if !rowMatchesFilter(v.filter, h.ID, h.Project, h.Job, h.RefFilter, h.Reason) {
			continue
		}
		if filtered == idx {
			idx = i
			break
		}
		filtered++
	}
	if idx >= len(v.holds) {
		return
	}

	hold := v.holds[idx]
	bg := tcell.NewRGBColor(24, 24, 32)
	muted := tcell.NewRGBColor(120, 120, 140)
	deleteBg := tcell.NewRGBColor(200, 50, 50)
	deleteActiveBg := tcell.NewRGBColor(235, 87, 87)
	cancelBg := tcell.NewRGBColor(60, 60, 75)
	cancelActiveBg := tcell.NewRGBColor(80, 80, 100)

	title := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	title.SetBackgroundColor(bg)
	fmt.Fprint(title, "[bold][red]Delete Autohold Request[-][-]")

	details := tview.NewTextView().SetDynamicColors(true)
	details.SetBackgroundColor(bg)
	fmt.Fprintf(details, "   [::b]ID:[-:-:-]       [white]%s[-]\n", hold.ID)
	fmt.Fprintf(details, "   [::b]Project:[-:-:-]  [white]%s[-]\n", hold.Project)
	fmt.Fprintf(details, "   [::b]Job:[-:-:-]      [white]%s[-]", hold.Job)
	detailRows := 3
	if hold.RefFilter != "" {
		fmt.Fprintf(details, "\n   [::b]Ref:[-:-:-]      [white]%s[-]", hold.RefFilter)
		detailRows++
	}
	if hold.Reason != "" {
		fmt.Fprintf(details, "\n   [::b]Reason:[-:-:-]   [white]%s[-]", hold.Reason)
		detailRows++
	}

	warning := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	warning.SetBackgroundColor(bg)
	warning.SetTextColor(muted)
	fmt.Fprint(warning, "This action cannot be undone.")

	statusBar := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	statusBar.SetBackgroundColor(bg)

	form := tview.NewForm()
	form.SetBackgroundColor(bg)
	form.SetButtonsAlign(tview.AlignCenter)

	deleteActiveStyle := tcell.StyleDefault.Background(deleteActiveBg).Foreground(tcell.ColorWhite)
	cancelActiveStyle := tcell.StyleDefault.Background(cancelActiveBg).Foreground(tcell.ColorWhite)

	form.SetButtonActivatedStyle(deleteActiveStyle)

	form.AddButton("Delete", func() {
		statusBar.Clear()
		fmt.Fprint(statusBar, "[yellow]Deleting...[-]")
		go func() {
			err := v.client.DeleteAutohold(hold.ID)
			v.app.QueueUpdateDraw(func() {
				if err != nil {
					statusBar.Clear()
					fmt.Fprintf(statusBar, "[red]Error: %v[-]", err)
					return
				}
				v.closeConfirm()
				v.Load(v.client)
			})
		}()
	})
	deleteBtn := form.GetButton(0)
	deleteBtn.SetBackgroundColor(deleteBg)

	form.AddButton("Cancel", func() {
		v.closeConfirm()
	})
	cancelBtn := form.GetButton(1)
	cancelBtn.SetBackgroundColor(cancelBg)

	form.SetCancelFunc(func() {
		v.closeConfirm()
	})

	focusIdx := 0
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			focusIdx = (focusIdx + 1) % 2
		case tcell.KeyBacktab:
			focusIdx = (focusIdx + 1) % 2
		}
		if focusIdx == 1 {
			form.SetButtonActivatedStyle(cancelActiveStyle)
		} else {
			form.SetButtonActivatedStyle(deleteActiveStyle)
		}
		return event
	})

	hint := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	hint.SetBackgroundColor(bg)
	fmt.Fprint(hint, "[blue]tab[-:-:-][::d]:switch[-:-:-]  [blue]enter[-:-:-][::d]:confirm[-:-:-]  [blue]esc[-:-:-][::d]:cancel[-:-:-]")

	spacer := func() *tview.Box { return tview.NewBox().SetBackgroundColor(bg) }

	inner := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(spacer(), 1, 0, false).
		AddItem(title, 1, 0, false).
		AddItem(spacer(), 2, 0, false).
		AddItem(details, detailRows, 0, false).
		AddItem(spacer(), 2, 0, false).
		AddItem(warning, 1, 0, false).
		AddItem(spacer(), 2, 0, false).
		AddItem(form, 1, 0, true).
		AddItem(spacer(), 1, 0, false).
		AddItem(statusBar, 1, 0, false).
		AddItem(hint, 1, 0, false).
		AddItem(spacer(), 1, 0, false)
	inner.SetBackgroundColor(bg)

	dialogHeight := detailRows + 13
	dialog := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(spacer(), 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(spacer(), 0, 1, false).
			AddItem(inner, 65, 0, true).
			AddItem(spacer(), 0, 1, false),
			dialogHeight, 0, true).
		AddItem(spacer(), 0, 1, false)
	dialog.SetBackgroundColor(bg)

	v.modal = true
	v.pages.AddAndSwitchToPage("confirm", dialog, true)
	v.app.SetFocus(form)
}
