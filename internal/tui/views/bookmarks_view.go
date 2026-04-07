package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
)

type BookmarksView struct {
	root    *tview.Flex
	table   *tview.Table
	logView *BuildLogView
	pages   *tview.Pages
	app     *tview.Application
	manager *BookmarkManager
	client  *api.Client
	onDetail bool
}

func NewBookmarksView(app *tview.Application, manager *BookmarkManager, dlManager *DownloadManager, aiCfg config.AIConfig) *BookmarksView {
	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(ColorBg)
	table.SetSelectedStyle(SelectedStyle)

	keys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(ColorNavBg)
	fmt.Fprint(keys, " [#3884f4]enter[-:-:-][::d]:open build[-:-:-]  [#3884f4]c[-:-:-][::d]:change[-:-:-]  [#3884f4]d[-:-:-][::d]:remove[-:-:-]  [#3884f4]o[-:-:-][::d]:open web[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:navigate[-:-:-]")

	tableWithKeys := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	tableWithKeys.SetBackgroundColor(ColorBg)

	logView := NewBuildLogView(app, dlManager, aiCfg)

	pages := tview.NewPages().
		AddPage("table", tableWithKeys, true, true).
		AddPage("detail", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(ColorBg)

	v := &BookmarksView{
		root:    root,
		table:   table,
		logView: logView,
		pages:   pages,
		app:     app,
		manager: manager,
	}

	manager.SetOnChange(func() {
		if !v.onDetail {
			v.renderTable()
		}
	})

	table.SetSelectedFunc(func(row, _ int) {
		rec := v.selectedRecord()
		if rec == nil || v.client == nil {
			return
		}
		fallback := v.recordToBuild(rec)
		if fallback.Result == "" && fallback.UUID != "" {
			v.logView.StreamBuild(v.client, fallback)
			v.pages.SwitchToPage("detail")
			v.onDetail = true
			return
		}
		v.logView.ShowStaticLog(v.client, fallback)
		v.pages.SwitchToPage("detail")
		v.onDetail = true
		go func() {
			build, err := v.client.GetBuild(rec.UUID)
			if err != nil || build == nil {
				return
			}
			v.app.QueueUpdateDraw(func() {
				v.logView.ShowStaticLog(v.client, build)
			})
		}()
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		rec := v.selectedRecord()
		if rec == nil {
			return event
		}
		switch {
		case event.Rune() == 'd':
			manager.Remove(rec.UUID)
			return nil
		case event.Rune() == 'o':
			if v.client != nil {
				openURL(v.client.BuildURL(rec.UUID))
			}
			return nil
		case event.Rune() == 'c':
			if rec.RefURL != "" {
				openURL(rec.RefURL)
			}
			return nil
		}
		return event
	})

	logView.SetBookmarkManager(manager)
	logView.SetBackHandler(func() {
		v.pages.SwitchToPage("table")
		v.onDetail = false
		v.renderTable()
	})

	return v
}

func (v *BookmarksView) Root() tview.Primitive { return v.root }

func (v *BookmarksView) Load(client *api.Client) {
	v.client = client
	if !v.onDetail {
		v.renderTable()
	}
	v.refreshFromAPI(client)
}

func (v *BookmarksView) currentHost() string {
	if v.client != nil {
		return v.client.Host()
	}
	return ""
}

func (v *BookmarksView) refreshFromAPI(client *api.Client) {
	if client == nil {
		return
	}
	records := v.filteredRecords()
	for _, rec := range records {
		uuid := rec.UUID
		go func() {
			build, err := client.GetBuild(uuid)
			if err != nil || build == nil {
				return
			}
			v.manager.Update(uuid, build)
		}()
	}
}

func (v *BookmarksView) IsModal() bool { return v.logView.IsAnalysisActive() }

func (v *BookmarksView) SetFilter(_ string) {}

func (v *BookmarksView) filteredRecords() []BookmarkRecord {
	host := v.currentHost()
	tenant := ""
	if v.client != nil {
		tenant = v.client.Tenant()
	}
	if host != "" || tenant != "" {
		return v.manager.RecordsByContext(host, tenant)
	}
	return v.manager.Records()
}

func (v *BookmarksView) selectedRecord() *BookmarkRecord {
	row, _ := v.table.GetSelection()
	idx := row - 1
	records := v.filteredRecords()
	if idx < 0 || idx >= len(records) {
		return nil
	}
	return &records[idx]
}

func (v *BookmarksView) recordToBuild(rec *BookmarkRecord) *api.Build {
	return &api.Build{
		UUID:      rec.UUID,
		JobName:   rec.JobName,
		Pipeline:  rec.Pipeline,
		Result:    rec.Result,
		StartTime: rec.StartTime,
		LogURL:    rec.LogURL,
		Ref: api.BuildRef{
			Project: rec.Project,
			Branch:  rec.Branch,
			RefURL:  rec.RefURL,
		},
	}
}

func (v *BookmarksView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Job", "Project", "Branch", "Instance", "Tenant", "Result", "Pipeline", "Change", "Saved")

	records := v.filteredRecords()
	if len(records) == 0 {
		v.table.SetCell(1, 0, tview.NewTableCell(" [::d]No bookmarks yet. Press b in a build detail to save one.[-:-:-]").SetSelectable(false))
		return
	}

	muted := ColorMuted
	dim := ColorDim
	for i, r := range records {
		row := i + 1
		rc := resultColor(r.Result)

		host := r.Host
		if host == "" {
			host = "—"
		}
		tenant := r.Tenant
		if tenant == "" {
			tenant = "—"
		}

		v.table.SetCell(row, 0, tview.NewTableCell(" "+resultIcon(r.Result)+" "+r.JobName).SetTextColor(rc).SetExpansion(1))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+r.Project).SetTextColor(muted).SetMaxWidth(40))
		v.table.SetCell(row, 2, tview.NewTableCell(" "+r.Branch).SetTextColor(muted).SetMaxWidth(15))
		v.table.SetCell(row, 3, tview.NewTableCell(" "+host).SetTextColor(dim).SetMaxWidth(30))
		v.table.SetCell(row, 4, tview.NewTableCell(" "+tenant).SetTextColor(muted).SetMaxWidth(25))
		v.table.SetCell(row, 5, resultCell(r.Result))
		v.table.SetCell(row, 6, tview.NewTableCell(" "+truncate(r.Pipeline, 20)).SetTextColor(muted))
		v.table.SetCell(row, 7, tview.NewTableCell(" "+r.Change).SetTextColor(ColorAccent))
		v.table.SetCell(row, 8, tview.NewTableCell(" "+formatDLDate(r.SavedAt)).SetTextColor(dim))
	}
}
