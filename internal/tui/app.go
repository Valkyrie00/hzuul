package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Valkyrie00/hzuul/internal/ai"
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
	"github.com/Valkyrie00/hzuul/internal/tui/views"
	"github.com/Valkyrie00/hzuul/internal/updater"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const defaultRefreshInterval = 30 * time.Second

type filterState struct {
	text string
	pos  int
}

type App struct {
	app             *tview.Application
	pages           *tview.Pages
	nav             *NavBar
	header          *tview.TextView
	keyBar          *views.KeyBar
	filterText      string
	filterPos       int
	filterOpen      bool
	filterTimer     *time.Timer //nolint:unused // used by scheduleFilter
	quitPending     bool
	client          *api.Client
	cfg             *config.Config
	version         string
	latestVersion   string
	views           []views.View
	dlManager       *views.DownloadManager
	bmManager       *views.BookmarkManager
	stopCh          chan struct{}
	refreshInterval time.Duration
	savedFilters    map[int]filterState
	viewLoaded      map[int]bool
}

func New(cfg *config.Config, version string) (*App, error) {
	ctx, err := cfg.Active()
	if err != nil {
		return nil, err
	}

	tview.Styles.PrimitiveBackgroundColor = tcell.ColorDefault
	tview.Styles.ContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.MoreContrastBackgroundColor = tcell.ColorDefault
	tview.Styles.BorderColor = tcell.ColorDefault
	tview.Styles.TitleColor = tcell.ColorDefault
	tview.Styles.GraphicsColor = tcell.ColorDefault
	tview.Styles.PrimaryTextColor = tcell.ColorDefault
	tview.Styles.SecondaryTextColor = tcell.ColorDefault
	tview.Styles.TertiaryTextColor = tcell.ColorDefault
	tview.Styles.InverseTextColor = tcell.ColorDefault
	tview.Styles.ContrastSecondaryTextColor = tcell.ColorDefault

	a := &App{
		app:             tview.NewApplication(),
		pages:           tview.NewPages(),
		cfg:             cfg,
		version:         version,
		stopCh:          make(chan struct{}),
		refreshInterval: defaultRefreshInterval,
		savedFilters:    make(map[int]filterState),
		viewLoaded:      make(map[int]bool),
	}

	a.dlManager = views.NewDownloadManager(a.app)
	a.bmManager = views.NewBookmarkManager()
	a.header = a.buildHeader(ctx)
	a.keyBar = views.NewKeyBar()
	a.keyBar.SetGlobalHints(views.GlobalHints())
	a.keyBar.SetHints(nil)
	a.nav = NewNavBar(a.switchView)
	a.views = a.buildViews()

	a.bmManager.SetOnChange(func() {
		if su, ok := a.views[a.nav.Active()].(views.StatusUpdater); ok {
			su.UpdateStatus()
		}
	})

	a.dlManager.SetOnChange(func() {
		count := a.dlManager.ActiveDownloadCount()
		if count > 0 {
			a.nav.SetBadge(9, fmt.Sprintf("%d", count))
		} else {
			a.nav.SetBadge(9, "")
		}
		if su, ok := a.views[a.nav.Active()].(views.StatusUpdater); ok {
			su.UpdateStatus()
		}
		a.updateDownloadProgress()
	})

	for i, v := range a.views {
		a.pages.AddPage(tabNames[i], v.Root(), true, i == 0)
	}

	loadingText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	loadingText.SetBackgroundColor(views.ColorBg)
	a.pages.AddPage("loading", loadingText, true, true)

	navSpacer := tview.NewBox()

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 1, 0, false).
		AddItem(a.nav, 1, 0, false).
		AddItem(navSpacer, 1, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.keyBar.Root(), 1, 0, false)

	a.app.SetRoot(layout, true)
	a.app.SetInputCapture(a.globalInput)

	go a.initClient(ctx, loadingText)
	go a.checkForUpdates()

	return a, nil
}

func (a *App) initClient(ctx *config.Context, loadingText *tview.TextView) {
	setStatus := func(msg string) {
		a.app.QueueUpdateDraw(func() {
			loadingText.Clear()
			_, _ = fmt.Fprintf(loadingText, "\n\n\n [yellow]%s[-]", msg)
		})
	}

	client, err := api.NewClient(ctx, setStatus)
	if err != nil {
		a.app.QueueUpdateDraw(func() {
			loadingText.Clear()
			_, _ = fmt.Fprintf(loadingText, "\n\n\n [red::b]Error:[-:-:-] %v\n\n [::d]Press q to quit[-:-:-]", err)
		})
		return
	}

	a.app.QueueUpdateDraw(func() {
		a.client = client
		a.pages.RemovePage("loading")
		a.pages.SwitchToPage(tabNames[0])
		a.views[0].Load(a.client)
		a.viewLoaded[0] = true
		a.renderKeyBar()
		a.updateFooterTime()
		go a.autoRefresh()
	})
}

func (a *App) Run() error {
	defer close(a.stopCh)
	return a.app.Run()
}

func (a *App) aiLabel() string {
	label := ai.ProviderLabel(a.cfg.AI)
	if label == "" {
		return "[::d]│[-:-:-] [::d]ai:[-:-:-] [red]not configured[-:-:-]"
	}
	color := "green"
	if !ai.HasAnalyzer(a.cfg.AI) {
		color = "yellow"
	}
	return fmt.Sprintf("[::d]│[-:-:-] [::d]ai:[-:-:-] [%s]%s[-:-:-]", color, label)
}

func (a *App) buildHeader(ctx *config.Context) *tview.TextView {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(views.ColorHeaderBg)
	a.writeHeader(tv, ctx)
	return tv
}

func (a *App) writeHeader(tv *tview.TextView, ctx *config.Context) {
	tv.Clear()
	versionBadge := fmt.Sprintf("[::d]%s", strings.ToUpper(a.version))
	if a.latestVersion != "" {
		versionBadge += fmt.Sprintf(" [yellow](%s ![-:-:-][yellow])[-:-:-]", a.latestVersion)
	}
	versionBadge += "[-:-:-]"
	_, _ = fmt.Fprintf(tv, " [#3884F4::b]HZUUL[-:-:-] %s [::d]│[-:-:-] %s [::d]│[-:-:-] [::d]tenant:[-:-:-] [#e5c07b::b]%s[-:-:-] [::d]│[-:-:-] [::d]ctx:[-:-:-] [green]%s[-:-:-] %s",
		versionBadge, ctx.URL, ctx.Tenant, a.cfg.CurrentContext, a.aiLabel())
}

func (a *App) checkForUpdates() {
	res, err := updater.Check(a.version)
	if err != nil || !res.Available {
		return
	}
	ctx, err := a.cfg.Active()
	if err != nil {
		return
	}
	a.app.QueueUpdateDraw(func() {
		a.latestVersion = res.Latest
		a.writeHeader(a.header, ctx)
	})
}

func (a *App) renderKeyBar() {
	if a.quitPending {
		a.keyBar.SetMessage("[yellow::b]Quit HZUUL?[-:-:-]  [#48c78e::b]y[-:-:-][::d]:yes[-:-:-]  [#eb5757::b]n[-:-:-][::d]:no[-:-:-]")
		return
	}
	if a.filterOpen {
		runes := []rune(a.filterText)
		before := string(runes[:a.filterPos])
		after := ""
		cursor := " "
		if a.filterPos < len(runes) {
			cursor = string(runes[a.filterPos])
			after = string(runes[a.filterPos+1:])
		}
		a.keyBar.SetMessage(fmt.Sprintf("[#3884f4]/[-][white]%s[-][black:white]%s[-:-][white]%s[-]", before, cursor, after))
		return
	}

	var hints []views.KeyHint
	idx := a.nav.Active()
	if hp, ok := a.views[idx].(views.KeyHintProvider); ok {
		hints = hp.KeyHints()
	}
	if hints == nil {
		return
	}
	if a.filterText != "" {
		a.keyBar.SetHintsWithFilter(a.filterText, hints)
	} else {
		a.keyBar.SetHints(hints)
	}
}

//nolint:unused // used by scheduleFilter
func (a *App) cancelFilterTimer() {
	if a.filterTimer != nil {
		a.filterTimer.Stop()
		a.filterTimer = nil
	}
}

//nolint:unused // reserved for future live-filter debouncing
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

func (a *App) isLiveFilterable() bool {
	idx := a.nav.Active()
	if lf, ok := a.views[idx].(views.LiveFilterable); ok {
		return lf.IsLiveFilterable()
	}
	return false
}

func (a *App) buildViews() []views.View {
	aiCfg := a.cfg.AI
	kb := a.keyBar
	vv := []views.View{
		views.NewStatusView(a.app, kb, a.dlManager, aiCfg),
		views.NewProjectsView(a.app, kb, a.dlManager, aiCfg),
		views.NewJobsView(a.app, kb, a.dlManager, aiCfg),
		views.NewLabelsView(a.app, kb),
		views.NewNodesView(a.app, kb),
		views.NewAutoholdsView(a.app, kb),
		views.NewSemaphoresView(a.app, kb),
		views.NewBuildsView(a.app, kb, a.dlManager, aiCfg),
		views.NewBuildsetsView(a.app, kb, a.dlManager, aiCfg),
		views.NewDownloadsView(a.app, kb, a.dlManager, aiCfg),
		views.NewBookmarksView(a.app, kb, a.bmManager, a.dlManager, aiCfg),
	}
	for _, v := range vv {
		if bv, ok := v.(views.BookmarkAwareView); ok {
			bv.SetBookmarkManager(a.bmManager)
		}
	}
	return vv
}

func (a *App) switchView(index int) {
	if index < 0 || index >= len(tabNames) || a.client == nil {
		return
	}
	old := a.nav.Active()
	a.savedFilters[old] = filterState{text: a.filterText, pos: a.filterPos}

	saved := a.savedFilters[index]
	a.filterText = saved.text
	a.filterPos = saved.pos
	a.filterOpen = false

	a.pages.SwitchToPage(tabNames[index])
	if !a.viewLoaded[index] {
		a.views[index].Load(a.client)
		a.viewLoaded[index] = true
	}
	if su, ok := a.views[index].(views.StatusUpdater); ok {
		su.UpdateStatus()
	} else {
		a.keyBar.ClearStatus()
	}
	a.renderKeyBar()
	a.updateFooterTime()
}

func (a *App) resetAllViews() {
	for i := range a.views {
		a.views[i].SetFilter("")
		delete(a.viewLoaded, i)
		delete(a.savedFilters, i)
	}
	a.filterText = ""
	a.filterPos = 0
	a.renderKeyBar()
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
	a.keyBar.SetTimestamp(time.Now().Format("15:04:05"))
}

func (a *App) updateDownloadProgress() {
	var active, done, total int
	for _, r := range a.dlManager.Records() {
		if r.Status == views.DLDownloading {
			active++
			done += r.DoneFiles
			total += r.TotalFiles
		}
	}
	if active == 0 {
		a.keyBar.SetDownloadProgress("")
		return
	}
	pct := 0
	if total > 0 {
		pct = done * 100 / total
	}
	if active == 1 {
		a.keyBar.SetDownloadProgress(fmt.Sprintf("[yellow::b]↓[-:-:-][::d] %d%% (%d/%d)[-:-:-]", pct, done, total))
	} else {
		a.keyBar.SetDownloadProgress(fmt.Sprintf("[yellow::b]↓%d[-:-:-][::d] %d%% (%d/%d)[-:-:-]", active, pct, done, total))
	}
}

func (a *App) globalInput(event *tcell.EventKey) *tcell.EventKey {
	if mv, ok := a.views[a.nav.Active()].(views.ModalView); ok && mv.IsModal() {
		return event
	}

	if a.quitPending {
		switch event.Rune() {
		case 'y', 'Y':
			a.app.Stop()
		default:
			a.quitPending = false
			a.renderKeyBar()
		}
		return nil
	}

	if a.client == nil {
		if event.Rune() == 'q' {
			a.app.Stop()
		}
		return nil
	}

	if a.filterOpen {
		runes := []rune(a.filterText)
		switch event.Key() {
		case tcell.KeyEsc:
			a.filterOpen = false
			a.filterText = ""
			a.filterPos = 0
			a.applyFilter()
			a.renderKeyBar()
			return nil
		case tcell.KeyEnter:
			a.filterOpen = false
			a.applyFilter()
			a.renderKeyBar()
			return nil
		case tcell.KeyLeft:
			if a.filterPos > 0 {
				a.filterPos--
				a.renderKeyBar()
			}
			return nil
		case tcell.KeyRight:
			if a.filterPos < len(runes) {
				a.filterPos++
				a.renderKeyBar()
			}
			return nil
		case tcell.KeyHome:
			a.filterPos = 0
			a.renderKeyBar()
			return nil
		case tcell.KeyEnd:
			a.filterPos = len(runes)
			a.renderKeyBar()
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if a.filterPos > 0 {
				a.filterText = string(append(runes[:a.filterPos-1], runes[a.filterPos:]...))
				a.filterPos--
				a.renderKeyBar()
				if a.isLiveFilterable() {
					a.applyFilter()
				}
			}
			return nil
		case tcell.KeyDelete:
			if a.filterPos < len(runes) {
				a.filterText = string(append(runes[:a.filterPos], runes[a.filterPos+1:]...))
				a.renderKeyBar()
				if a.isLiveFilterable() {
					a.applyFilter()
				}
			}
			return nil
		case tcell.KeyRune:
			a.filterText = string(append(runes[:a.filterPos], append([]rune{event.Rune()}, runes[a.filterPos:]...)...))
			a.filterPos++
			a.renderKeyBar()
			if a.isLiveFilterable() {
				a.applyFilter()
			}
			return nil
		}
		return nil
	}

	if a.nav.HandleKey(event) {
		return nil
	}

	switch event.Rune() {
	case 'q':
		a.quitPending = true
		a.renderKeyBar()
		return nil
	case 'r':
		idx := a.nav.Active()
		if rc, ok := a.views[idx].(views.Reconnectable); ok && rc.CanReconnect() {
			rc.Reconnect()
			return nil
		}
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
		a.renderKeyBar()
		return nil
	}

	return event
}

func (a *App) showHelp() {
	helpText := ` [::b]Keybindings[-:-:-]

 [#3884f4]Navigation[-:-:-]
   1-9, 0      Switch to view (0=Downloads)
   b           Bookmarks
   Tab         Next view
   Shift+Tab   Previous view

 [#3884f4]Actions[-:-:-]
   r           Refresh current view
   t           Change tenant
   Enter       Open detail (in tables)
   l           Stream log (in Builds)
   d           Download logs (in Build detail)
   s           Save bookmark (in Build detail)
   c           Open change/MR/PR (in Build detail)
   q / Esc     Quit / Back

 [#3884f4]Analysis[-:-:-]
   a           AI failure analysis (Build detail / Downloads)

 [#3884f4]Admin (requires token)[-:-:-]
   x           Dequeue change (Status/Builds)
   p           Promote change (Status)

 [#3884f4]Tables[-:-:-]
   Up/Down     Navigate rows
   /           Filter (Enter to apply, Esc to clear)
               Builds/Buildsets: server-side
               job:x  project:x  pipeline:x
               branch:x  result:x  change:x  uuid:x
               Smart: org/repo → project, text → job

 [#78788c]Auto-refresh: every 30 seconds[-:-:-]`

	modal := tview.NewTextView().
		SetDynamicColors(true).
		SetText(helpText)
	modal.SetBorder(true).
		SetTitle(" Help ").
		SetBorderColor(views.ColorSep)

	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			a.dismissModal("help")
			return nil
		}
		return event
	})

	a.pages.AddAndSwitchToPage("help", center(modal, 50, 29), true)
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
			list.SetBorderColor(views.ColorSep)
			list.SetMainTextColor(tcell.ColorWhite)
			list.SetSecondaryTextColor(views.ColorMuted)
			list.SetSelectedTextColor(tcell.ColorWhite)
			list.SetSelectedBackgroundColor(views.ColorSelectBg)
			list.SetHighlightFullLine(true)
			list.ShowSecondaryText(false)

			currentTenant := a.client.Tenant()

			for i, t := range tenants {
				name := t.Name
				prefix := "  "
				if name == currentTenant {
					prefix = "● "
					list.SetCurrentItem(i)
				}
				list.AddItem(prefix+name, "", 0, func() {
					a.client.SetTenant(name)
					ctx, _ := a.cfg.Active()
					a.writeHeader(a.header, ctx)
					a.dismissModal("tenants")
					a.resetAllViews()
					idx := a.nav.Active()
					a.views[idx].Load(a.client)
					a.viewLoaded[idx] = true
				})
			}

			list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
					a.dismissModal("tenants")
					return nil
				}
				return event
			})

			height := len(tenants)*2 + 2
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
		SetText(fmt.Sprintf("\n [#eb5757::b]Error: %s[-:-:-]\n\n [#78788c]%v[-:-:-]\n\n [#78788c]Press [white]q[-:-:-][#78788c] or [white]Esc[-:-:-][#78788c] to close[-:-:-]", context, err))
	text.SetBorder(true).
		SetTitle(" Error ").
		SetBorderColor(views.ColorSep)
	text.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			a.dismissModal("error")
			return nil
		}
		return event
	})
	a.pages.AddAndSwitchToPage("error", center(text, 60, 10), true)
}

func (a *App) dismissModal(name string) {
	a.pages.RemovePage(name)
	a.pages.SwitchToPage(tabNames[a.nav.Active()])
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
