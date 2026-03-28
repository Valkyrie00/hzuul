package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
	"github.com/vcastell/hzuul/internal/config"
	"github.com/vcastell/hzuul/internal/tui/views"
)

const defaultRefreshInterval = 30 * time.Second

type App struct {
	app    *tview.Application
	pages  *tview.Pages
	nav    *NavBar
	header     *tview.TextView
	footer     *tview.Flex
	footerKeys *tview.TextView
	footerTime *tview.TextView
	filterText  string
	filterPos   int
	filterOpen  bool
	filterTimer *time.Timer
	client  *api.Client
	cfg     *config.Config
	views   []views.View
	stopCh  chan struct{}
	refreshInterval time.Duration
}

func New(cfg *config.Config) (*App, error) {
	ctx, err := cfg.Active()
	if err != nil {
		return nil, err
	}

	client, err := api.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating API client: %w", err)
	}

	a := &App{
		app:             tview.NewApplication(),
		pages:           tview.NewPages(),
		cfg:             cfg,
		client:          client,
		stopCh:          make(chan struct{}),
		refreshInterval: defaultRefreshInterval,
	}

	a.header = a.buildHeader(ctx)
	a.buildFooter()
	a.nav = NewNavBar(a.switchView)
	a.views = a.buildViews()

	for i, v := range a.views {
		a.pages.AddPage(tabNames[i], v.Root(), true, i == 0)
	}

	navSpacer := tview.NewBox().SetBackgroundColor(ColorBg)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.nav, 1, 0, false).
		AddItem(navSpacer, 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.footer, 1, 0, false)

	a.app.SetRoot(layout, true)
	a.app.SetInputCapture(a.globalInput)

	return a, nil
}

func (a *App) Run() error {
	a.views[0].Load(a.client)
	go a.autoRefresh()
	defer close(a.stopCh)
	return a.app.Run()
}

func (a *App) buildHeader(ctx *config.Context) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(ColorHeaderBg)
	fmt.Fprintf(tv, " [#3884F4::b]HZUUL[-:-:-] [::d]│[-] %s [::d]│[-] [::d]tenant:[-] [white::b]%s[-:-:-] [::d]│[-] [::d]ctx:[-] [green]%s[-]",
		ctx.URL, ctx.Tenant, a.cfg.CurrentContext)
	return tv
}

func (a *App) buildFooter() {
	keys := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(ColorNavBg)

	ts := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)
	ts.SetBackgroundColor(ColorNavBg)

	a.footerKeys = keys
	a.footerTime = ts
	a.updateFooterKeysText()

	a.footer = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(keys, 0, 1, false).
		AddItem(ts, 22, 0, false)
}

const footerKeysBase = " [#3884f4]?[-:-:-][::d]:help[-:-:-]  [#3884f4]t[-:-:-][::d]:tenant[-:-:-]  [#3884f4]r[-:-:-][::d]:refresh[-:-:-]  [#3884f4]1-9[-:-:-][::d]:views[-:-:-]  [#3884f4]/[-:-:-][::d]:filter[-:-:-]  [#3884f4]q[-:-:-][::d]:quit[-:-:-]"

func (a *App) updateFooterKeysText() {
	a.footerKeys.Clear()
	if a.filterOpen {
		runes := []rune(a.filterText)
		before := string(runes[:a.filterPos])
		after := ""
		cursor := " "
		if a.filterPos < len(runes) {
			cursor = string(runes[a.filterPos])
			after = string(runes[a.filterPos+1:])
		}
		fmt.Fprintf(a.footerKeys, " [#3884f4]/[-][white]%s[-][black:white]%s[-:-][white]%s[-]", before, cursor, after)
	} else if a.filterText != "" {
		fmt.Fprintf(a.footerKeys, " [#3884f4]/[-][white]%s[-]  %s", a.filterText, footerKeysBase[1:])
	} else {
		fmt.Fprint(a.footerKeys, footerKeysBase)
	}
}

func (a *App) cancelFilterTimer() {
	if a.filterTimer != nil {
		a.filterTimer.Stop()
		a.filterTimer = nil
	}
}

func (a *App) scheduleFilter() {
	a.cancelFilterTimer()
	a.filterTimer = time.AfterFunc(500*time.Millisecond, func() {
		a.app.QueueUpdateDraw(func() {
			a.applyFilter()
		})
	})
}

func (a *App) applyFilter() {
	idx := a.nav.Active()
	a.views[idx].SetFilter(a.filterText)
}

func (a *App) buildViews() []views.View {
	return []views.View{
		views.NewStatusView(a.app),
		views.NewProjectsView(a.app),
		views.NewJobsView(a.app),
		views.NewLabelsView(a.app),
		views.NewNodesView(a.app),
		views.NewAutoholdsView(a.app),
		views.NewSemaphoresView(a.app),
		views.NewBuildsView(a.app),
		views.NewBuildsetsView(a.app),
	}
}

func (a *App) switchView(index int) {
	if index < 0 || index >= len(tabNames) {
		return
	}
	a.cancelFilterTimer()
	old := a.nav.Active()
	if old >= 0 && old < len(a.views) {
		a.views[old].SetFilter("")
	}
	a.filterOpen = false
	a.filterText = ""
	a.filterPos = 0
	a.updateFooterKeysText()

	a.pages.SwitchToPage(tabNames[index])
	a.views[index].Load(a.client)
	a.updateFooterTime()
}

func (a *App) autoRefresh() {
	ticker := time.NewTicker(a.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			if a.filterText != "" {
				continue
			}
			idx := a.nav.Active()
			a.views[idx].Load(a.client)
			a.app.QueueUpdateDraw(func() {
				a.updateFooterTime()
			})
		}
	}
}

func (a *App) updateFooterTime() {
	a.footerTime.Clear()
	fmt.Fprintf(a.footerTime, "[::d]last update: %s [-]", time.Now().Format("15:04:05"))
}

func (a *App) globalInput(event *tcell.EventKey) *tcell.EventKey {
	if mv, ok := a.views[a.nav.Active()].(views.ModalView); ok && mv.IsModal() {
		return event
	}

	if a.filterOpen {
		runes := []rune(a.filterText)
		switch event.Key() {
		case tcell.KeyEsc:
			a.cancelFilterTimer()
			a.filterOpen = false
			a.filterText = ""
			a.filterPos = 0
			a.applyFilter()
			a.updateFooterKeysText()
			return nil
		case tcell.KeyEnter:
			a.cancelFilterTimer()
			a.filterOpen = false
			a.applyFilter()
			a.updateFooterKeysText()
			return nil
		case tcell.KeyLeft:
			if a.filterPos > 0 {
				a.filterPos--
				a.updateFooterKeysText()
			}
			return nil
		case tcell.KeyRight:
			if a.filterPos < len(runes) {
				a.filterPos++
				a.updateFooterKeysText()
			}
			return nil
		case tcell.KeyHome:
			a.filterPos = 0
			a.updateFooterKeysText()
			return nil
		case tcell.KeyEnd:
			a.filterPos = len(runes)
			a.updateFooterKeysText()
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if a.filterPos > 0 {
				a.filterText = string(append(runes[:a.filterPos-1], runes[a.filterPos:]...))
				a.filterPos--
				a.updateFooterKeysText()
			}
			return nil
		case tcell.KeyDelete:
			if a.filterPos < len(runes) {
				a.filterText = string(append(runes[:a.filterPos], runes[a.filterPos+1:]...))
				a.updateFooterKeysText()
			}
			return nil
		case tcell.KeyRune:
			a.filterText = string(append(runes[:a.filterPos], append([]rune{event.Rune()}, runes[a.filterPos:]...)...))
			a.filterPos++
			a.updateFooterKeysText()
			return nil
		}
		return nil
	}

	if a.nav.HandleKey(event) {
		return nil
	}

	switch event.Rune() {
	case 'q':
		a.showQuitConfirm()
		return nil
	case 'r':
		idx := a.nav.Active()
		a.views[idx].Load(a.client)
		a.updateFooterTime()
		return nil
	case '?':
		a.showHelp()
		return nil
	case 't':
		a.showTenantPicker()
		return nil
	case '/':
		a.filterOpen = true
		a.filterPos = len([]rune(a.filterText))
		a.updateFooterKeysText()
		return nil
	}

	return event
}

func (a *App) showHelp() {
	helpText := `[bold]hZuul Keybindings[-]

[#3884f4]Navigation[-]
  1-9         Switch to view
  Tab         Next view
  Shift+Tab   Previous view

[#3884f4]Actions[-]
  r           Refresh current view
  t           Change tenant
  Enter       Open detail (in tables)
  l           Stream log (in Builds)
  q / Esc     Quit / Back

[#3884f4]Tables[-]
  Up/Down     Navigate rows
  /           Search (Esc to clear)
              Builds/Buildsets: server-side
              job:x  project:x  pipeline:x
              branch:x  result:x  change:x

[#3884f4]General[-]
  ?           This help
  q           Quit application

[::d]Auto-refresh: every 30 seconds[-]`

	modal := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText)
	modal.SetBackgroundColor(ColorBg)
	modal.SetBorder(true).
		SetTitle(" Help ").
		SetBorderColor(ColorAccent)

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			a.pages.RemovePage("help")
			return nil
		}
		return event
	})

	a.pages.AddAndSwitchToPage("help", center(modal, 50, 22), true)
}

func (a *App) showTenantPicker() {
	go func() {
		tenants, err := a.client.GetTenants()
		if err != nil {
			a.app.QueueUpdateDraw(func() {
				a.showError("Loading tenants", err)
			})
			return
		}

		a.app.QueueUpdateDraw(func() {
			list := tview.NewList()
			list.SetTitle(" Select Tenant ").SetBorder(true)
			list.SetBackgroundColor(ColorBg)
			list.SetBorderColor(ColorAccent)
			list.SetMainTextColor(tcell.ColorWhite)

			for _, t := range tenants {
				name := t.Name
				list.AddItem(name, "", 0, func() {
					a.client.SetTenant(name)
					a.header.Clear()
					ctx, _ := a.cfg.Active()
					fmt.Fprintf(a.header, " [#3884F4::b]HZUUL[-:-:-] [::d]│[-] %s [::d]│[-] [::d]tenant:[-] [white::b]%s[-:-:-] [::d]│[-] [::d]ctx:[-] [green]%s[-]",
						ctx.URL, name, a.cfg.CurrentContext)
					a.pages.RemovePage("tenants")
					a.views[a.nav.Active()].Load(a.client)
				})
			}

			list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
					a.pages.RemovePage("tenants")
					return nil
				}
				return event
			})

			height := len(tenants) + 2
			if height > 20 {
				height = 20
			}
			a.pages.AddAndSwitchToPage("tenants", center(list, 50, height), true)
		})
	}()
}

func (a *App) showError(context string, err error) {
	text := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[red][bold]Error: %s[-][-]\n\n%v", context, err))
	text.SetBackgroundColor(ColorBg)
	text.SetBorder(true).
		SetTitle(" Error ").
		SetBorderColor(ColorFailure)
	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			a.pages.RemovePage("error")
			return nil
		}
		return event
	})
	a.pages.AddAndSwitchToPage("error", center(text, 60, 8), true)
}

func (a *App) showQuitConfirm() {
	modal := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("\n\n[bold]Quit HZUUL?[-]\n\n[::d]Press [white]y[-:-:-] to confirm or [white]n[-:-:-]/[white]Esc[-:-:-] to cancel[-:-:-]")
	modal.SetBackgroundColor(ColorBg)
	modal.SetBorder(true).
		SetTitle(" Quit ").
		SetBorderColor(ColorAccent)

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'y', 'Y':
			a.app.Stop()
			return nil
		case 'n', 'N', 'q':
			a.pages.RemovePage("quit")
			return nil
		}
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("quit")
			return nil
		}
		return event
	})

	a.pages.AddAndSwitchToPage("quit", center(modal, 40, 8), true)
}

func center(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}
