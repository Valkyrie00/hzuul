package views

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type BuildLogView struct {
	root        *tview.Flex
	textView    *tview.TextView
	header      *tview.TextView
	app         *tview.Application
	streamer    *api.LogStreamer
	mu          sync.Mutex
	stopCh      chan struct{}
	openURL     string
	logURL      string
	baseContent string
}

func NewBuildLogView(app *tview.Application) *BuildLogView {
	bg := tcell.NewRGBColor(24, 24, 32)
	dimColor := tcell.NewRGBColor(50, 50, 65)

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
	fmt.Fprint(separator, "  ──────────────────────────────────────")

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() { app.Draw() })
	textView.SetBackgroundColor(bg)
	textView.SetBorderPadding(0, 0, 2, 2)

	navBg := tcell.NewRGBColor(32, 32, 44)
	keys := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(navBg)
	fmt.Fprint(keys, " [blue]esc[-:-:-][::d]:back[-:-:-]  [blue]o[-:-:-][::d]:open web[-:-:-]  [blue]l[-:-:-][::d]:open logs[-:-:-]  [blue]↑↓[-:-:-][::d]:scroll[-:-:-]")

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(separator, 1, 0, false).
		AddItem(textView, 0, 1, true).
		AddItem(keys, 1, 0, false)
	root.SetBackgroundColor(bg)

	return &BuildLogView{
		root:     root,
		textView: textView,
		header:   header,
		app:      app,
	}
}

func (v *BuildLogView) Root() tview.Primitive { return v.root }

func (v *BuildLogView) Load(_ *api.Client) {}

func (v *BuildLogView) StreamBuild(client *api.Client, build *api.Build) {
	v.Stop()
	v.logURL = build.LogURL
	v.openURL = build.LogURL

	v.header.Clear()
	fmt.Fprintf(v.header, " [bold]Log[-] │ [blue]%s[-] │ %s │ %s",
		build.JobName, build.Ref.Project, build.Ref.Branch)

	v.textView.Clear()
	fmt.Fprintln(v.textView, "[::d]Connecting to log stream...[-:-:-]")

	v.stopCh = make(chan struct{})

	go func() {
		streamer, err := client.StreamLog(build.UUID, "console.log")
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.textView.Clear()
				fmt.Fprintf(v.textView, "[red]Stream error: %v[-]\n\n", err)
				fmt.Fprintf(v.textView, "[::d]Log URL: %s[-:-:-]\n", build.LogURL)
			})
			return
		}

		v.mu.Lock()
		v.streamer = streamer
		v.mu.Unlock()

		v.app.QueueUpdateDraw(func() {
			v.textView.Clear()
		})

		var buf strings.Builder
		var bufMu sync.Mutex
		done := make(chan struct{})

		go func() {
			defer close(done)
			for {
				msg, err := streamer.ReadMessage()
				if err != nil {
					bufMu.Lock()
					buf.WriteString("\n[::d]--- stream ended ---[-:-:-]\n")
					bufMu.Unlock()
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
				return
			case <-done:
				bufMu.Lock()
				remaining := buf.String()
				buf.Reset()
				bufMu.Unlock()
				if remaining != "" {
					v.app.QueueUpdateDraw(func() {
						fmt.Fprint(v.textView, remaining)
						v.textView.ScrollToEnd()
					})
				}
				return
			case <-ticker.C:
				bufMu.Lock()
				chunk := buf.String()
				buf.Reset()
				bufMu.Unlock()
				if chunk != "" {
					v.app.QueueUpdateDraw(func() {
						fmt.Fprint(v.textView, chunk)
						v.textView.ScrollToEnd()
					})
				}
			}
		}
	}()
}

func (v *BuildLogView) ShowStaticLog(client *api.Client, build *api.Build) {
	v.Stop()
	v.logURL = build.LogURL
	if client != nil {
		v.openURL = client.BuildURL(build.UUID)
	} else {
		v.openURL = build.LogURL
	}
	v.header.Clear()
	fmt.Fprintf(v.header, " [bold]Build Detail[-] │ [blue]%s[-] │ %s │ %s",
		build.JobName, build.Ref.Project, build.Ref.Branch)

	thinLine := "────────────────────────────────────────────────────────────────────────────────"

	var b strings.Builder
	fmt.Fprintf(&b, "[bold]Job:[-]       %s\n", build.JobName)
	fmt.Fprintf(&b, "[bold]UUID:[-]      %s\n", build.UUID)
	fmt.Fprintf(&b, "[bold]Project:[-]   %s\n", build.Ref.Project)
	fmt.Fprintf(&b, "[bold]Branch:[-]    %s\n", build.Ref.Branch)
	if build.Ref.Change != nil && build.Ref.Patchset != nil {
		fmt.Fprintf(&b, "[bold]Change:[-]    %v,%v\n", build.Ref.Change, build.Ref.Patchset)
	} else if build.Ref.Ref != "" {
		fmt.Fprintf(&b, "[bold]Ref:[-]       %s\n", build.Ref.Ref)
	}
	fmt.Fprintf(&b, "[bold]Result:[-]    %s\n", resultTag(build.Result))
	fmt.Fprintf(&b, "[bold]Duration:[-]  %s\n", formatBuildDuration(build.Duration))
	fmt.Fprintf(&b, "[bold]Start:[-]     %s\n", build.StartTime)
	fmt.Fprintf(&b, "[bold]End:[-]       %s\n", build.EndTime)
	fmt.Fprintf(&b, "[bold]Voting:[-]    %v\n", build.Voting)
	fmt.Fprintf(&b, "[bold]Nodeset:[-]   %s\n", build.Nodeset)
	if build.LogURL != "" {
		fmt.Fprintf(&b, "[bold]Log URL:[-]   %s\n", build.LogURL)
	}
	if client != nil {
		fmt.Fprintf(&b, "[bold]Web URL:[-]   %s\n", client.BuildURL(build.UUID))
	}
	if build.ErrorDetail != "" {
		fmt.Fprintf(&b, "\n[red][bold]Error:[-] %s[-]\n", build.ErrorDetail)
	}
	if len(build.Artifacts) > 0 {
		fmt.Fprintf(&b, "\n[::d]%s[-:-:-]\n", thinLine)
		fmt.Fprintf(&b, "[bold]  Artifacts[-:-:-]\n")
		fmt.Fprintf(&b, "[::d]%s[-:-:-]\n", thinLine)
		for _, a := range build.Artifacts {
			fmt.Fprintf(&b, "  • %s: %s\n", a.Name, a.URL)
		}
	}

	v.baseContent = b.String()
	v.textView.Clear()
	fmt.Fprint(v.textView, v.baseContent)

	if client != nil && build.LogURL != "" {
		fmt.Fprintf(v.textView, "\n[::d] Loading task summary...[-:-:-]")
		go v.fetchTaskSummary(client, build.LogURL)
	}
}

func (v *BuildLogView) fetchTaskSummary(client *api.Client, logURL string) {
	output, err := client.GetJobOutput(logURL)
	if err != nil {
		v.app.QueueUpdateDraw(func() {
			v.textView.Clear()
			fmt.Fprint(v.textView, v.baseContent)
			fmt.Fprintf(v.textView, "\n[::d]  ⚠ Could not load task summary: %v[-:-:-]\n", err)
		})
		return
	}

	stats := api.AggregateStats(output)
	failed := api.ExtractFailedTasks(output, stats)

	if len(stats) == 0 && len(failed) == 0 {
		v.app.QueueUpdateDraw(func() {
			v.textView.Clear()
			fmt.Fprint(v.textView, v.baseContent)
			fmt.Fprintf(v.textView, "\n[::d]  ✓ No task failures detected[-:-:-]\n")
		})
		return
	}

	v.app.QueueUpdateDraw(func() {
		v.textView.Clear()
		fmt.Fprint(v.textView, v.baseContent)
		thickLine := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

		if len(stats) > 0 {
			fmt.Fprintf(v.textView, "\n[blue]%s[-]\n", thickLine)
			fmt.Fprintf(v.textView, "[bold][blue]  Task Summary[-]\n")
			fmt.Fprintf(v.textView, "[blue]%s[-]\n\n", thickLine)

			nameW := 4
			for host := range stats {
				if len(host) > nameW {
					nameW = len(host)
				}
			}
			if nameW > 40 {
				nameW = 40
			}

			fmt.Fprintf(v.textView, "  [::b]  %-*s  %5s  %5s  %5s  %5s  %5s[-]\n",
				nameW, "HOST", "OK", "FAIL", "CHGD", "SKIP", "UNRCH")

			for host, s := range stats {
				display := host
				if len(display) > nameW {
					display = display[:nameW-1] + "…"
				}
				indicator := "[green]●[-]"
				if s.Failures > 0 {
					indicator = "[red]●[-]"
				}
				failColor := ""
				failEnd := ""
				if s.Failures > 0 {
					failColor = "[red]"
					failEnd = "[-]"
				}
				fmt.Fprintf(v.textView, "  %s %-*s  [green]%5d[-]  %s%5d%s  [yellow]%5d[-]  [::d]%5d[-:-:-]  [::d]%5d[-:-:-]\n",
					indicator, nameW, display, s.Ok, failColor, s.Failures, failEnd, s.Changed, s.Skipped, s.Unreachable)
			}
		}

		if len(failed) > 0 {
			fmt.Fprintf(v.textView, "\n[red]%s[-]\n", thickLine)
			fmt.Fprintf(v.textView, "[bold][red]  Errors (%d)[-]\n", len(failed))
			fmt.Fprintf(v.textView, "[red]%s[-]\n", thickLine)

			for i, ft := range failed {
				fmt.Fprintf(v.textView, "\n\n  [red][bold]ERROR %d/%d[-][-]\n", i+1, len(failed))
				fmt.Fprintf(v.textView, "  [red][bold]✕[-][-] Task [bold]%s[-]  failed running on host [bold]%s[-]\n", ft.Task, ft.Host)

				if ft.Cmd != "" {
					fmt.Fprintln(v.textView)
					fmt.Fprintf(v.textView, "  [::d]Command:[-:-:-]\n")
					for _, line := range wrapText(ft.Cmd, 72) {
						fmt.Fprintf(v.textView, "    [::d]%s[-:-:-]\n", line)
					}
				}

				if ft.Msg != "" {
					fmt.Fprintln(v.textView)
					fmt.Fprintf(v.textView, "  [bold]Reason:[-]  [yellow]%s[-]\n", ft.Msg)
				}

				output := ft.Stdout
				if output == "" {
					output = ft.Stderr
				}
				if output != "" {
					lines := strings.Split(output, "\n")
					maxPreview := 30
					if len(lines) > maxPreview {
						lines = lines[len(lines)-maxPreview:]
					}
					fmt.Fprintln(v.textView)
					fmt.Fprintf(v.textView, "  [bold]Output:[-]\n")
					fmt.Fprintf(v.textView, "  [::d]────────────────────────────────────────────────────────────[-:-:-]\n")
					for _, line := range lines {
						if len(line) > 120 {
							line = line[:120] + "…"
						}
						fmt.Fprintf(v.textView, "    %s\n", line)
					}
					fmt.Fprintf(v.textView, "  [::d]────────────────────────────────────────────────────────────[-:-:-]\n")
				}

				if i < len(failed)-1 {
					fmt.Fprintf(v.textView, "\n\n[red]%s[-]\n", strings.Repeat("━", 80))
				}
			}
		}
	})
}

func wrapText(s string, width int) []string {
	if len(s) <= width {
		return []string{s}
	}
	var lines []string
	for len(s) > width {
		cut := width
		if sp := strings.LastIndex(s[:cut], " "); sp > width/3 {
			cut = sp
		}
		lines = append(lines, s[:cut])
		s = strings.TrimLeft(s[cut:], " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

func formatBuildDuration(v any) string {
	if v == nil {
		return "—"
	}
	var secs float64
	switch d := v.(type) {
	case float64:
		secs = d
	case int:
		secs = float64(d)
	case json.Number:
		secs, _ = d.Float64()
	default:
		s := fmt.Sprintf("%v", v)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			secs = f
		} else {
			return s
		}
	}
	total := int(secs)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh %d min %d secs", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%d min %d secs", m, s)
	}
	return fmt.Sprintf("%d secs", s)
}

func resultTag(result string) string {
	switch result {
	case "SUCCESS":
		return "[green]SUCCESS[-]"
	case "FAILURE", "ERROR":
		return "[red]" + result + "[-]"
	case "LOST", "ABORTED", "TIMED_OUT":
		return "[yellow]" + result + "[-]"
	default:
		return "[blue]" + result + "[-]"
	}
}

func (v *BuildLogView) Stop() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.stopCh != nil {
		close(v.stopCh)
		v.stopCh = nil
	}
	if v.streamer != nil {
		v.streamer.Close()
		v.streamer = nil
	}
}
