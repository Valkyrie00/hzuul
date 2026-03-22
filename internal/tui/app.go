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
	app      *tview.Application
	pages    *tview.Pages
	nav      *NavBar
	header   *tview.TextView
	footer   *tview.TextView
	client   *api.Client
	cfg      *config.Config
	views    []views.View
	stopCh   chan struct{}
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
	a.footer = a.buildFooter()
	a.nav = NewNavBar(a.switchView)
	a.views = a.buildViews()

	for i, v := range a.views {
		a.pages.AddPage(tabNames[i], v.Root(), true, i == 0)
	}

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.nav, 1, 0, false).
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
	fmt.Fprintf(tv, " [bold::b]hZuul[-:-:-] │ [::d]%s[-:-:-] │ tenant: [blue]%s[-] │ context: [green]%s[-]",
		ctx.URL, ctx.Tenant, a.cfg.CurrentContext)
	return tv
}

func (a *App) buildFooter() *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(ColorHeaderBg)
	fmt.Fprint(tv, " [::d]q[-]:quit  [::d]?[-]:help  [::d]t[-]:tenant  [::d]r[-]:refresh  [::d]1-9[-]:views  [::d]/[-]:filter")
	return tv
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
			a.app.QueueUpdateDraw(func() {
				idx := a.nav.Active()
				a.views[idx].Load(a.client)
				a.updateFooterTime()
			})
		}
	}
}

func (a *App) updateFooterTime() {
	a.footer.Clear()
	fmt.Fprintf(a.footer, " [::d]q[-]:quit  [::d]?[-]:help  [::d]t[-]:tenant  [::d]r[-]:refresh  [::d]1-9[-]:views  [::d]/[-]:filter │ [::d]last update: %s[-]",
		time.Now().Format("15:04:05"))
}

func (a *App) globalInput(event *tcell.EventKey) *tcell.EventKey {
	if a.nav.HandleKey(event) {
		return nil
	}

	switch event.Rune() {
	case 'q':
		a.app.Stop()
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
	}

	return event
}

func (a *App) showHelp() {
	helpText := `[bold]hZuul Keybindings[-]

[blue]Navigation[-]
  1-9         Switch to view
  Tab         Next view
  Shift+Tab   Previous view

[blue]Actions[-]
  r           Refresh current view
  t           Change tenant
  Enter       Open detail (in tables)
  l           Stream log (in Builds)
  q / Esc     Quit / Back

[blue]Tables[-]
  Up/Down     Navigate rows
  /           Filter (future)

[blue]General[-]
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
					fmt.Fprintf(a.header, " [bold::b]hZuul[-:-:-] │ [::d]%s[-:-:-] │ tenant: [blue]%s[-] │ context: [green]%s[-]",
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

func center(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 0, true).
			AddItem(nil, 0, 1, false), width, 0, true).
		AddItem(nil, 0, 1, false)
}
