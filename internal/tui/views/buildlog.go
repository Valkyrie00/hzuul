package views

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Valkyrie00/hzuul/internal/ai"
	"github.com/Valkyrie00/hzuul/internal/api"
	"github.com/Valkyrie00/hzuul/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type BuildLogView struct {
	root         *tview.Flex
	textView     *tview.TextView
	header       *tview.TextView
	separator    *tview.TextView
	keys         *tview.TextView
	pathInput    *tview.InputField
	app          *tview.Application
	streamer     *api.LogStreamer
	mu           sync.Mutex
	stopCh       chan struct{}
	buildWebURL  string
	logURL       string
	contentFlex  *tview.Flex
	infoView     *tview.TextView
	errorsHeader *tview.TextView

	client         *api.Client
	build          *api.Build
	dlManager      *DownloadManager
	bmManager      *BookmarkManager
	onBack         func()
	isStatic       bool
	inputActive    bool
	dequeuePending bool
	streamDead     bool
	headerTicker   *time.Ticker
	streamStart    time.Time

	jobOutput   []api.PlaybookOutput
	failedTasks []api.FailedTask

	pages       *tview.Pages
	buildLayout *tview.Flex
	analysis    *AnalysisPanel
}

func NewBuildLogView(app *tview.Application, dlManager *DownloadManager, aiCfg config.AIConfig) *BuildLogView {
	bg := ColorBg
	dimColor := ColorSep

	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	header.SetBackgroundColor(bg)
	header.SetBorderPadding(0, 0, 2, 0)

	separator := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	separator.SetBackgroundColor(bg)
	separator.SetTextColor(dimColor)
	_, _ = fmt.Fprint(separator, "  ──────────────────────────────────────")

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() { app.Draw() })
	textView.SetBackgroundColor(bg)
	textView.SetBorderPadding(0, 0, 2, 2)

	navBg := ColorNavBg
	keys := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(navBg)
	_, _ = fmt.Fprint(keys, " [#3884f4]esc[-:-:-][::d]:back[-:-:-]  [#3884f4]s[-:-:-][::d]:toggle bookmark[-:-:-]  [#3884f4]c[-:-:-][::d]:open change[-:-:-]  [#3884f4]o[-:-:-][::d]:open web[-:-:-]  [#3884f4]↑↓[-:-:-][::d]:scroll[-:-:-]")

	pathInput := tview.NewInputField()
	pathInput.SetBackgroundColor(navBg)
	pathInput.SetFieldBackgroundColor(navBg)
	pathInput.SetFieldTextColor(tcell.ColorWhite)
	pathInput.SetLabelColor(ColorAccent)
	pathInput.SetLabel(" Download to: ")

	infoView := tview.NewTextView().SetDynamicColors(true)
	infoView.SetBackgroundColor(bg)
	infoView.SetBorderPadding(0, 0, 2, 2)

	errorsHeader := tview.NewTextView().SetDynamicColors(true)
	errorsHeader.SetBackgroundColor(bg)
	errorsHeader.SetBorderPadding(0, 0, 2, 2)

	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(bg)
	contentFlex.AddItem(textView, 0, 1, true)

	buildLayout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(separator, 1, 0, false).
		AddItem(contentFlex, 0, 1, true).
		AddItem(keys, 1, 0, false)
	buildLayout.SetBackgroundColor(bg)

	panel := NewAnalysisPanel(app, aiCfg)

	pages := tview.NewPages().
		AddPage("build", buildLayout, true, true).
		AddPage("analysis", panel.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(bg)

	return &BuildLogView{
		root:         root,
		textView:     textView,
		contentFlex:  contentFlex,
		infoView:     infoView,
		errorsHeader: errorsHeader,
		header:       header,
		separator:    separator,
		keys:         keys,
		pathInput:    pathInput,
		app:          app,
		dlManager:    dlManager,
		pages:        pages,
		buildLayout:  buildLayout,
		analysis:     panel,
	}
}

func (v *BuildLogView) Root() tview.Primitive { return v.root }

func (v *BuildLogView) IsAnalysisActive() bool { return v.analysis.IsActive() }

func (v *BuildLogView) CanReconnect() bool {
	return !v.isStatic && v.streamDead && v.client != nil && v.build != nil
}

func (v *BuildLogView) Reconnect() {
	if v.CanReconnect() {
		v.StreamBuild(v.client, v.build)
	}
}

func (v *BuildLogView) updateKeys() {
	v.keys.Clear()
	if v.dequeuePending && v.build != nil {
		_, _ = fmt.Fprintf(v.keys, " [red::b]Dequeue[-:-:-] [white]%s[-] [::d]from %s[-:-:-]  [#48c78e::b]y[-:-:-][::d]:confirm[-:-:-]  [#eb5757::b]n[-:-:-][::d]:cancel[-:-:-]",
			truncate(v.build.JobName, 30), v.build.Pipeline)
		return
	}
	if v.isStatic {
		base := " [#3884f4]esc[-:-:-][::d]:back[-:-:-]  [#3884f4]s[-:-:-][::d]:toggle bookmark[-:-:-]  [#3884f4]d[-:-:-][::d]:download[-:-:-]  [#3884f4]c[-:-:-][::d]:open change[-:-:-]  [#3884f4]o[-:-:-][::d]:open web[-:-:-]  [#3884f4]l[-:-:-][::d]:open logs[-:-:-]"
		if v.build != nil && v.build.Result != "SUCCESS" && v.build.Result != "SKIPPED" {
			base += "  [#e5c07b]a[-:-:-][::d]:AI analysis[-:-:-]"
		}
		base += "  [#3884f4]↑↓[-:-:-][::d]:scroll[-:-:-]"
		_, _ = fmt.Fprint(v.keys, base)
	} else {
		base := " [#3884f4]esc[-:-:-][::d]:back[-:-:-]  [#3884f4]s[-:-:-][::d]:toggle bookmark[-:-:-]  [#3884f4]c[-:-:-][::d]:open change[-:-:-]  [#3884f4]o[-:-:-][::d]:open web[-:-:-]"
		if v.streamDead {
			base += "  [yellow]r[-:-:-][::d]:reconnect[-:-:-]"
		}
		if v.client != nil && v.client.HasAdminToken() {
			base += "  [#3884f4]x[-:-:-][::d]:dequeue[-:-:-]"
		}
		base += "  [#3884f4]↑↓[-:-:-][::d]:scroll[-:-:-]"
		_, _ = fmt.Fprint(v.keys, base)
	}
}

func (v *BuildLogView) flashKeys(msg string) {
	v.keys.Clear()
	_, _ = fmt.Fprint(v.keys, " "+msg)
	go func() {
		time.Sleep(2 * time.Second)
		v.app.QueueUpdateDraw(func() {
			v.updateKeys()
		})
	}()
}

func (v *BuildLogView) SetBookmarkManager(bm *BookmarkManager) {
	v.bmManager = bm
}

func (v *BuildLogView) Load(_ *api.Client) {}

func (v *BuildLogView) SetBackHandler(onBack func()) {
	v.onBack = onBack

	v.analysis.SetOnExit(func() {
		v.pages.SwitchToPage("build")
		v.app.SetFocus(v.textView)
		v.updateKeys()
		v.updateBookmarkHeader(false)

		v.mu.Lock()
		jobOutput := v.jobOutput
		failedTasks := v.failedTasks
		v.mu.Unlock()

		if jobOutput != nil {
			stats := api.AggregateStats(jobOutput)
			v.renderBuildDetail(stats, failedTasks, "")
		} else {
			v.renderBuildDetail(nil, nil, "")
		}
	})

	v.root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if v.analysis.IsActive() {
			return v.analysis.HandleKey(event)
		}
		if v.inputActive {
			return event
		}
		if v.dequeuePending {
			switch event.Rune() {
			case 'y', 'Y':
				v.executeDequeue()
			case 'n', 'N':
				v.dequeuePending = false
				v.updateKeys()
			}
			return nil
		}
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.Stop()
			onBack()
			return nil
		}
		if event.Rune() == 'o' {
			if v.buildWebURL != "" {
				openURL(v.buildWebURL)
			} else {
				v.flashKeys("[yellow]No web URL available for this build[-]")
			}
			return nil
		}
		if event.Rune() == 'l' && v.logURL != "" {
			openURL(v.logURL)
			return nil
		}
		if event.Rune() == 'c' {
			if v.build != nil && v.build.Ref.RefURL != "" {
				openURL(v.build.Ref.RefURL)
			} else {
				v.flashKeys("[yellow]No change URL available for this build[-]")
			}
			return nil
		}
		if event.Rune() == 's' && v.bmManager != nil && v.build != nil {
			added := v.bmManager.Toggle(v.client, v.build)
			v.updateBookmarkHeader(added)
			return nil
		}
		downloading := v.build != nil && v.dlManager != nil && v.dlManager.IsDownloading(v.build.UUID)
		if event.Rune() == 'd' && v.isStatic && !downloading && v.dlManager != nil && v.build != nil && v.build.LogURL != "" {
			v.showPathPrompt()
			return nil
		}
		if event.Rune() == 'x' && !v.isStatic && v.client != nil && v.client.HasAdminToken() && v.build != nil {
			v.dequeuePending = true
			v.updateKeys()
			return nil
		}
		if event.Rune() == 'a' && v.isStatic && v.build != nil && v.build.Result != "SUCCESS" && v.build.Result != "SKIPPED" {
			v.startAnalysis()
			return nil
		}
		return event
	})
}

func (v *BuildLogView) executeDequeue() {
	if v.build == nil || v.client == nil {
		return
	}
	v.keys.Clear()
	_, _ = fmt.Fprint(v.keys, " [yellow::b]Dequeuing...[-:-:-]")

	build := v.build
	go func() {
		req := &api.DequeueRequest{
			Pipeline: build.Pipeline,
			Project:  build.Ref.Project,
		}
		if build.Ref.Change != nil && build.Ref.Patchset != nil {
			req.Change = fmt.Sprintf("%v,%v", build.Ref.Change, build.Ref.Patchset)
		} else if build.Ref.Ref != "" {
			req.Ref = build.Ref.Ref
		}
		err := v.client.Dequeue(build.Ref.Project, req)
		v.app.QueueUpdateDraw(func() {
			v.dequeuePending = false
			if err != nil {
				v.keys.Clear()
				_, _ = fmt.Fprintf(v.keys, " [red]Error: %v[-]", err)
				return
			}
			v.keys.Clear()
			_, _ = fmt.Fprint(v.keys, " [green]Dequeued successfully[-]")
		})
	}()
}

func (v *BuildLogView) bookmarkTag() string {
	if v.bmManager != nil && v.build != nil && v.bmManager.IsBookmarked(v.build.UUID) {
		return " │ [yellow]★ saved[-]"
	}
	return ""
}

func (v *BuildLogView) headerElapsed() string {
	start := v.streamStart
	if v.build != nil && v.build.StartTime != "" {
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05.000000",
		} {
			if t, err := time.Parse(layout, v.build.StartTime); err == nil {
				start = t
				break
			}
		}
	}
	if start.IsZero() {
		return ""
	}
	d := time.Since(start)
	if d < 0 {
		return ""
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("[green::b]%dh %02dm %02ds[-:-:-]", h, m, s)
	}
	return fmt.Sprintf("[green::b]%dm %02ds[-:-:-]", m, s)
}

func (v *BuildLogView) headerRefInfo() string {
	b := v.build
	if b.Ref.Change != nil {
		return fmt.Sprintf("#%v", b.Ref.Change)
	}
	return b.Ref.Branch
}

func (v *BuildLogView) renderHeader() {
	if v.build == nil {
		return
	}
	v.header.Clear()
	elapsed := v.headerElapsed()
	bookmark := v.bookmarkTag()
	elapsedPart := ""
	if elapsed != "" {
		elapsedPart = " │ " + elapsed
	}
	_, _ = fmt.Fprintf(v.header, "[bold]Stream[-] │ [#3884f4]%s[-] │ %s │ %s%s%s",
		v.build.JobName, v.build.Ref.Project, v.headerRefInfo(), elapsedPart, bookmark)
}

func (v *BuildLogView) startHeaderTicker() {
	v.stopHeaderTicker()
	v.headerTicker = time.NewTicker(1 * time.Second)
	go func() {
		for range v.headerTicker.C {
			v.app.QueueUpdateDraw(func() {
				v.renderHeader()
			})
		}
	}()
}

func (v *BuildLogView) stopHeaderTicker() {
	if v.headerTicker != nil {
		v.headerTicker.Stop()
		v.headerTicker = nil
	}
}

func (v *BuildLogView) updateBookmarkHeader(added bool) {
	if v.build == nil {
		return
	}
	if v.isStatic {
		v.infoView.Clear()
		for _, line := range v.buildInfoLines() {
			_, _ = fmt.Fprintln(v.infoView, line)
		}
		return
	}
	v.renderHeader()
}

func (v *BuildLogView) defaultDownloadDir() string {
	host := ""
	tenant := ""
	if v.client != nil {
		host = v.client.Host()
		tenant = v.client.Tenant()
	}
	uuid := ""
	if v.build != nil {
		uuid = v.build.UUID
	}
	return DefaultDownloadDir(host, tenant, uuid)
}

func (v *BuildLogView) showPathPrompt() {
	v.inputActive = true
	v.pathInput.SetText(v.defaultDownloadDir())

	v.pathInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			dest := strings.TrimSpace(v.pathInput.GetText())
			if dest == "" {
				return
			}
			v.hidePathPrompt()
			v.startDownload(dest)
		case tcell.KeyEsc:
			v.hidePathPrompt()
		}
	})

	v.buildLayout.RemoveItem(v.keys)
	v.buildLayout.AddItem(v.pathInput, 1, 0, true)
	v.app.SetFocus(v.pathInput)
}

func (v *BuildLogView) hidePathPrompt() {
	v.inputActive = false
	v.buildLayout.RemoveItem(v.pathInput)
	v.buildLayout.AddItem(v.keys, 1, 0, false)
	v.app.SetFocus(v.textView)
}

func (v *BuildLogView) startDownload(destDir string) {
	v.dlManager.Start(v.client, v.build, destDir, nil)
	if v.onBack != nil {
		v.onBack()
	}
}

func (v *BuildLogView) StreamBuild(client *api.Client, build *api.Build) {
	v.Stop()

	v.mu.Lock()
	v.jobOutput = nil
	v.failedTasks = nil
	v.mu.Unlock()

	v.logURL = build.LogURL
	if client != nil && build.UUID != "" {
		v.buildWebURL = client.StreamURL(build.UUID)
	} else {
		v.buildWebURL = build.LogURL
	}
	v.client = client
	v.build = build
	v.isStatic = false
	v.updateKeys()

	v.contentFlex.Clear()
	v.contentFlex.AddItem(v.textView, 0, 1, true)

	// Restore header + separator for Log streaming view
	v.buildLayout.RemoveItem(v.header)
	v.buildLayout.RemoveItem(v.separator)
	v.buildLayout.RemoveItem(v.contentFlex)
	v.buildLayout.RemoveItem(v.keys)
	v.buildLayout.AddItem(v.header, 1, 0, false)
	v.buildLayout.AddItem(v.separator, 1, 0, false)
	v.buildLayout.AddItem(v.contentFlex, 0, 1, true)
	v.buildLayout.AddItem(v.keys, 1, 0, false)

	v.streamStart = time.Now()
	v.renderHeader()
	v.startHeaderTicker()

	v.streamDead = false
	v.textView.Clear()
	_, _ = fmt.Fprintln(v.textView, "[::d]Connecting to log stream...[-:-:-]")

	v.stopCh = make(chan struct{})

	go v.streamLoop(client, build)
}

func (v *BuildLogView) streamLoop(client *api.Client, build *api.Build) {
	const maxRetries = 5
	const retryDelay = 3 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			v.app.QueueUpdateDraw(func() {
				_, _ = fmt.Fprintf(v.textView, "\n[yellow::b]Reconnecting... (attempt %d/%d)[-:-:-]\n", attempt, maxRetries)
				v.textView.ScrollToEnd()
			})

			select {
			case <-v.stopCh:
				return
			case <-time.After(retryDelay):
			}
		}

		streamer, err := client.StreamLog(build.UUID, "console.log")
		if err != nil {
			if attempt == maxRetries {
				v.app.QueueUpdateDraw(func() {
					_, _ = fmt.Fprintf(v.textView, "\n[red]Stream error: %v[-]\n", err)
					_, _ = fmt.Fprintf(v.textView, "[::d]Log URL: %s[-:-:-]\n", build.LogURL)
				})
				return
			}
			continue
		}

		v.mu.Lock()
		v.streamer = streamer
		v.mu.Unlock()

		if attempt == 0 {
			v.app.QueueUpdateDraw(func() {
				v.textView.Clear()
			})
		}

		disconnected := v.readStream(streamer)

		v.mu.Lock()
		v.streamer = nil
		v.mu.Unlock()
		_ = streamer.Close()

		if !disconnected {
			return
		}
	}

	v.stopHeaderTicker()
	v.app.QueueUpdateDraw(func() {
		v.streamDead = true
		_, _ = fmt.Fprintf(v.textView, "\n[red::b]Stream lost after %d retries[-:-:-]\n", maxRetries)
		_, _ = fmt.Fprint(v.textView, "[::d]Press [white::b]r[-:-:-][::d] to reconnect[-:-:-]\n")
		v.textView.ScrollToEnd()
		v.updateKeys()
	})
}

// readStream reads from the WebSocket and flushes to the UI.
// Returns true if the stream disconnected (should retry), false if stopped by user.
func (v *BuildLogView) readStream(streamer *api.LogStreamer) bool {
	var buf strings.Builder
	var bufMu sync.Mutex
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			msg, err := streamer.ReadMessage()
			if err != nil {
				return
			}
			bufMu.Lock()
			buf.WriteString(msg)
			bufMu.Unlock()
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-v.stopCh:
			return false
		case <-done:
			bufMu.Lock()
			remaining := buf.String()
			buf.Reset()
			bufMu.Unlock()
			if remaining != "" {
				colored := colorizeLogChunk(remaining)
				v.app.QueueUpdateDraw(func() {
					_, _ = fmt.Fprint(v.textView, colored)
					v.textView.ScrollToEnd()
				})
			}
			return true
		case <-ticker.C:
			bufMu.Lock()
			chunk := buf.String()
			buf.Reset()
			bufMu.Unlock()
			if chunk != "" {
				colored := colorizeLogChunk(chunk)
				v.app.QueueUpdateDraw(func() {
					_, _ = fmt.Fprint(v.textView, colored)
					v.textView.ScrollToEnd()
				})
			}
		}
	}
}

func (v *BuildLogView) ShowStaticLog(client *api.Client, build *api.Build) {
	v.Stop()

	v.mu.Lock()
	v.jobOutput = nil
	v.failedTasks = nil
	v.mu.Unlock()

	v.pages.SwitchToPage("build")
	v.textView.SetChangedFunc(nil)
	v.contentFlex.Clear()
	v.infoView.Clear()
	v.errorsHeader.Clear()
	v.textView.Clear()
	v.textView.SetChangedFunc(func() { v.app.Draw() })

	v.logURL = build.LogURL
	v.client = client
	v.build = build
	v.isStatic = true
	v.updateKeys()
	if client != nil {
		v.buildWebURL = client.BuildURL(build.UUID)
	} else {
		v.buildWebURL = build.LogURL
	}
	v.buildLayout.RemoveItem(v.header)
	v.buildLayout.RemoveItem(v.separator)

	loadMsg := ""
	if client != nil && build.LogURL != "" {
		loadMsg = "Loading task summary..."
		go v.fetchTaskSummary(client, build.LogURL)
	}
	v.renderBuildDetail(nil, nil, loadMsg)
}

func (v *BuildLogView) fetchTaskSummary(client *api.Client, logURL string) {
	output, err := client.GetJobOutput(logURL)

	v.app.QueueUpdateDraw(func() {
		if v.build == nil || v.build.LogURL != logURL {
			return
		}

		if err != nil {
			v.renderBuildDetail(nil, nil, fmt.Sprintf("⚠ Could not load task summary: %v", err))
			return
		}

		stats := api.AggregateStats(output)
		failed := api.ExtractFailedTasks(output, stats)

		v.mu.Lock()
		v.jobOutput = output
		v.failedTasks = failed
		v.mu.Unlock()

		v.renderBuildDetail(stats, failed, "")
	})
}

func (v *BuildLogView) renderBuildDetail(stats map[string]api.HostStats, failed []api.FailedTask, loadMsg string) {
	build := v.build
	if build == nil {
		return
	}

	v.textView.SetChangedFunc(nil)
	defer v.textView.SetChangedFunc(func() { v.app.Draw() })

	// Clear all persistent views first
	v.contentFlex.Clear()
	v.infoView.Clear()
	v.errorsHeader.Clear()
	v.textView.Clear()

	// --- Build Details (single column, fixed) ---
	var infoLines []string
	infoLines = append(infoLines, v.buildInfoLines()...)

	// Task Summary
	infoLines = append(infoLines,
		"",
		"[::b]Task Summary[-:-:-]",
		"[::d]──────────────────────────────────[-:-:-]",
		"",
	)
	summaryData := buildSummaryData(stats)
	if len(summaryData) > 0 {
		infoLines = append(infoLines, summaryData...)
	} else if loadMsg != "" {
		infoLines = append(infoLines, "[::d]"+loadMsg+"[-:-:-]")
	}

	for _, line := range infoLines {
		_, _ = fmt.Fprintln(v.infoView, line)
	}
	v.contentFlex.AddItem(v.infoView, len(infoLines), 0, false)

	// --- Errors header (fixed) + content (scrollable) ---
	if len(failed) > 0 {
		_, _ = fmt.Fprintf(v.errorsHeader, "\n[red::b]Errors[-:-:-]  [::d](%d)[-:-:-]    [#e5c07b]💡 press [white::b]a[-:-:-][#e5c07b] for AI analysis[-]\n", len(failed))
		_, _ = fmt.Fprintf(v.errorsHeader, "[::d]──────────────────────────────────[-:-:-]")
		v.contentFlex.AddItem(v.errorsHeader, 3, 0, false)
	} else if stats != nil && len(failed) == 0 {
		_, _ = fmt.Fprintf(v.errorsHeader, "\n[::d]✓ No task failures detected[-:-:-]")
		v.contentFlex.AddItem(v.errorsHeader, 2, 0, false)
	}

	w := v.textView

	if len(failed) > 0 {
		for i, ft := range failed {
			taskLine := fmt.Sprintf("\n[red]%d[-][::d]/%d[-:-:-] [red]✕[-] [white::b]%s[-:-:-] [#e5c07b]on[-] %s", i+1, len(failed), ft.Task, ft.Host)
			if ft.Msg != "" {
				taskLine += fmt.Sprintf(" [#e5c07b]return[-] [yellow]%s[-]", ft.Msg)
			}
			_, _ = fmt.Fprintln(w, taskLine)
			if ft.Cmd != "" {
				_, _ = fmt.Fprintf(w, "     [#e5c07b]cmd[-] [::d]$ %s[-:-:-]\n", truncateCmd(ft.Cmd, 120))
			}

			output := ft.Stdout
			if output == "" {
				output = ft.Stderr
			}
			if output != "" {
				lines := strings.Split(output, "\n")
				maxPreview := 15
				if len(lines) > maxPreview {
					lines = lines[len(lines)-maxPreview:]
					_, _ = fmt.Fprintf(w, "\n     [#e5c07b]output [-][::d](last %d lines)[-:-:-]\n", maxPreview)
				} else {
					_, _ = fmt.Fprintf(w, "\n     [#e5c07b]output[-:-:-]\n")
				}
				_, _ = fmt.Fprintf(w, "     [::d]%s[-:-:-]\n", strings.Repeat("═", 72))
				for _, line := range lines {
					if len(line) > 120 {
						line = line[:120] + "…"
					}
					_, _ = fmt.Fprintf(w, "     [::d]%s[-:-:-]\n", line)
				}
				_, _ = fmt.Fprintf(w, "     [::d]%s[-:-:-]\n", strings.Repeat("═", 72))
			}
		}
	}

	v.contentFlex.AddItem(v.textView, 0, 1, true)
	if !v.inputActive {
		v.app.SetFocus(v.textView)
	}
}

func (v *BuildLogView) buildInfoLines() []string {
	build := v.build
	row := func(name, value string) string {
		return fmt.Sprintf("[#78788c]%-12s[-]%s", name, value)
	}

	resultValue := fmt.Sprintf("%s  %s", resultEmoji(build.Result), resultTag(build.Result))
	if !isVoting(build.Voting) {
		resultValue += "  [yellow]non-voting[-]"
	}

	bookmarked := ""
	if v.bmManager != nil && v.bmManager.IsBookmarked(build.UUID) {
		bookmarked = "  [yellow]★ saved[-]"
	}

	lines := []string{
		"",
		"[::b]Build Details[-:-:-]",
		"[::d]──────────────────────────────────[-:-:-]",
		"",
		fmt.Sprintf("[#78788c]%-12s[-]%s%s", "Result", resultValue, bookmarked),
		row("Duration", formatBuildDuration(build.Duration)),
	}
	if build.ErrorDetail != "" {
		lines = append(lines, fmt.Sprintf("[#78788c]%-12s[-][red]%s[-]", "Error", build.ErrorDetail))
	}
	if isHeld(build.Held) {
		lines = append(lines, fmt.Sprintf("[#78788c]%-12s[-][yellow::b]NODE HELD[-:-:-]  [::d]for post-failure debugging[-:-:-]", "Held"))
	}

	lines = append(lines,
		"",
		row("UUID", build.UUID),
		fmt.Sprintf("[#78788c]%-12s[-][#3884f4]%s[-]", "Job", build.JobName),
		row("Project", build.Ref.Project),
		row("Branch", build.Ref.Branch),
	)
	if build.Ref.Change != nil && build.Ref.Patchset != nil {
		lines = append(lines, row("Change", fmt.Sprintf("%v,%v", build.Ref.Change, build.Ref.Patchset)))
	} else if build.Ref.Ref != "" {
		lines = append(lines, row("Ref", build.Ref.Ref))
	}
	if build.Ref.Newrev != "" {
		rev := build.Ref.Newrev
		if len(rev) > 10 {
			rev = rev[:10]
		}
		lines = append(lines, row("Revision", rev))
	}
	lines = append(lines,
		row("Pipeline", build.Pipeline),
		row("Nodeset", build.Nodeset),
		row("Start", formatTimestampFull(build.StartTime)),
		row("End", formatTimestampFull(build.EndTime)),
	)
	if build.Ref.RefURL != "" {
		lines = append(lines, fmt.Sprintf("[#78788c]%-12s[-][::d]%s[-:-:-]", "Change URL", build.Ref.RefURL))
	}
	if build.LogURL != "" {
		lines = append(lines, fmt.Sprintf("[#78788c]%-12s[-][::d]%s[-:-:-]", "Log URL", build.LogURL))
	}
	return lines
}

func buildSummaryData(stats map[string]api.HostStats) []string {
	if len(stats) == 0 {
		return nil
	}
	nameW := 4
	for host := range stats {
		if len(host) > nameW {
			nameW = len(host)
		}
	}
	if nameW > 18 {
		nameW = 18
	}
	lines := []string{
		fmt.Sprintf("[::d]  %-*s  %5s  %5s  %5s  %5s  %5s[-:-:-]", nameW, "HOST", "OK", "FAIL", "CHGD", "SKIP", "UNRCH"),
	}
	for host, s := range stats {
		display := host
		if len(display) > nameW {
			display = display[:nameW-1] + "…"
		}
		indicator := "[green]●[-]"
		if s.Failures > 0 {
			indicator = "[red]●[-]"
		}
		failStr := fmt.Sprintf("%5d", s.Failures)
		if s.Failures > 0 {
			failStr = fmt.Sprintf("[red]%5d[-]", s.Failures)
		}
		lines = append(lines, fmt.Sprintf("%s %-*s  [green]%5d[-]  %s  [yellow]%5d[-]  [::d]%5d[-:-:-]  [::d]%5d[-:-:-]",
			indicator, nameW, display, s.Ok, failStr, s.Changed, s.Skipped, s.Unreachable))
	}
	return lines
}

func truncateCmd(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func resultTag(result string) string {
	switch result {
	case "SUCCESS":
		return "[green::b]SUCCESS[-:-:-]"
	case "FAILURE", "ERROR":
		return "[red::b]" + result + "[-:-:-]"
	case "LOST", "ABORTED", "TIMED_OUT":
		return "[yellow::b]" + result + "[-:-:-]"
	default:
		return "[#3884f4::b]" + result + "[-:-:-]"
	}
}

func isVoting(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	}
	return true
}

func isHeld(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true" || val == "1"
	}
	return false
}

func formatTimestampFull(ts string) string {
	if ts == "" {
		return "—"
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05.000000",
	} {
		if t, err := time.Parse(layout, ts); err == nil {
			abs := t.Format("Mon 02 Jan 15:04")
			diff := time.Since(t)
			var rel string
			switch {
			case diff < time.Minute:
				rel = "just now"
			case diff < time.Hour:
				rel = fmt.Sprintf("%dm ago", int(diff.Minutes()))
			case diff < 24*time.Hour:
				rel = fmt.Sprintf("%dh %dm ago", int(diff.Hours()), int(diff.Minutes())%60)
			default:
				rel = fmt.Sprintf("%dd ago", int(diff.Hours()/24))
			}
			return fmt.Sprintf("%s  [::d](%s)[-:-:-]", abs, rel)
		}
	}
	if len(ts) > 16 {
		return ts[:16]
	}
	return ts
}

func resultEmoji(result string) string {
	switch result {
	case "SUCCESS":
		return "[green]✓[-]"
	case "FAILURE", "ERROR":
		return "[red]✕[-]"
	case "TIMED_OUT":
		return "[yellow]⏱[-]"
	case "LOST", "ABORTED":
		return "[yellow]⚠[-]"
	default:
		return "[#3884f4]●[-]"
	}
}

func (v *BuildLogView) startAnalysis() {
	if v.build == nil {
		return
	}

	v.analysis.Start(AnalysisBasic, v.build.JobName, v.build.Ref.Project)
	v.pages.SwitchToPage("analysis")
	v.app.SetFocus(v.analysis.Content())

	v.mu.Lock()
	jobOutput := v.jobOutput
	failedTasks := v.failedTasks
	v.mu.Unlock()

	pbSummaries := ai.PlaybookSummaries(jobOutput)
	classification := ai.ClassifyFailure(v.build.Result, failedTasks, pbSummaries)
	phase := ai.DetermineFailurePhase(pbSummaries)

	v.analysis.WriteClassification(classification, phase)

	w := v.analysis.Content()
	_, _ = fmt.Fprint(w, "  [::d] Fetching remote logs...[-:-:-]")

	client := v.client
	build := v.build

	go func() {
		da, _ := ai.ReadLogsFromRemote(client, build, jobOutput)

		v.app.QueueUpdateDraw(func() {
			if !v.analysis.IsActive() {
				return
			}

			var logContext []ai.LogBlock
			var logFiles []ai.LogFileSnippet
			var allFiles []string

			if da != nil {
				logContext = da.LogContext
				logFiles = da.LogFiles
				allFiles = da.AllFiles
				if len(da.FailedTasks) > 0 {
					failedTasks = da.FailedTasks
				}
			}

			if len(logFiles) > 0 {
				_, _ = fmt.Fprintf(w, " [bold]%d files fetched[-]\n", len(logFiles))
			} else {
				_, _ = fmt.Fprint(w, " [::d]no remote logs, using task output only[-:-:-]\n")
			}

			systemPrompt := ai.GetSystemPrompt()
			userPrompt := ai.BuildAnalysisPrompt(build, failedTasks, logContext)
			if len(logFiles) > 0 {
				input := ai.DirAnalysisInput{
					JobName: build.JobName,
					Project: build.Ref.Project,
				}
				enriched := &ai.DirAnalysis{
					JobOutput:   jobOutput,
					FailedTasks: failedTasks,
					LogContext:  logContext,
					LogFiles:    logFiles,
					AllFiles:    allFiles,
				}
				userPrompt = ai.BuildDirAnalysisPrompt(input, enriched)
			}

			v.analysis.StartAI(systemPrompt, userPrompt)
		})
	}()
}

func (v *BuildLogView) Stop() {
	v.stopHeaderTicker()

	v.mu.Lock()
	defer v.mu.Unlock()

	if v.stopCh != nil {
		close(v.stopCh)
		v.stopCh = nil
	}
	if v.streamer != nil {
		_ = v.streamer.Close()
		v.streamer = nil
	}
}
