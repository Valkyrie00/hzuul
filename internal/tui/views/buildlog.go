package views

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type BuildLogView struct {
	root     *tview.Flex
	textView *tview.TextView
	header   *tview.TextView
	app      *tview.Application
	streamer *api.LogStreamer
	mu       sync.Mutex
	stopCh   chan struct{}
	openURL  string
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
	fmt.Fprint(keys, " [blue]q[-][::d]:back[-]  [blue]o[-][::d]:open url[-]  [blue]↑↓[-][::d]:scroll[-]")

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
	v.openURL = build.LogURL

	v.header.Clear()
	fmt.Fprintf(v.header, " [bold]Log[-] │ [blue]%s[-] │ %s │ %s",
		build.JobName, build.Ref.Project, build.Ref.Branch)

	v.textView.Clear()
	fmt.Fprintln(v.textView, "[::d]Connecting to log stream...[-]")

	v.stopCh = make(chan struct{})

	go func() {
		streamer, err := client.StreamLog(build.UUID, "console.log")
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				v.textView.Clear()
				fmt.Fprintf(v.textView, "[red]Stream error: %v[-]\n\n", err)
				fmt.Fprintf(v.textView, "[::d]Log URL: %s[-]\n", build.LogURL)
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
					buf.WriteString("\n[::d]--- stream ended ---[-]\n")
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

func (v *BuildLogView) ShowStaticLog(build *api.Build) {
	v.Stop()
	v.openURL = build.LogURL
	v.header.Clear()
	fmt.Fprintf(v.header, " [bold]Build Detail[-] │ [blue]%s[-] │ %s │ %s",
		build.JobName, build.Ref.Project, build.Ref.Branch)

	v.textView.Clear()
	fmt.Fprintf(v.textView, "[bold]Job:[-]       %s\n", build.JobName)
	fmt.Fprintf(v.textView, "[bold]UUID:[-]      %s\n", build.UUID)
	fmt.Fprintf(v.textView, "[bold]Project:[-]   %s\n", build.Ref.Project)
	fmt.Fprintf(v.textView, "[bold]Branch:[-]    %s\n", build.Ref.Branch)
	fmt.Fprintf(v.textView, "[bold]Change:[-]    %v,%v\n", build.Ref.Change, build.Ref.Patchset)
	fmt.Fprintf(v.textView, "[bold]Result:[-]    %s\n", resultTag(build.Result))
	fmt.Fprintf(v.textView, "[bold]Duration:[-]  %v\n", build.Duration)
	fmt.Fprintf(v.textView, "[bold]Start:[-]     %s\n", build.StartTime)
	fmt.Fprintf(v.textView, "[bold]End:[-]       %s\n", build.EndTime)
	fmt.Fprintf(v.textView, "[bold]Voting:[-]    %v\n", build.Voting)
	fmt.Fprintf(v.textView, "[bold]Nodeset:[-]   %s\n", build.Nodeset)
	if build.LogURL != "" {
		fmt.Fprintf(v.textView, "[bold]Log URL:[-]   %s\n", build.LogURL)
	}
	if build.ErrorDetail != "" {
		fmt.Fprintf(v.textView, "\n[red][bold]Error:[-] %s[-]\n", build.ErrorDetail)
	}
	if len(build.Artifacts) > 0 {
		fmt.Fprintf(v.textView, "\n[bold]Artifacts:[-]\n")
		for _, a := range build.Artifacts {
			fmt.Fprintf(v.textView, "  • %s: %s\n", a.Name, a.URL)
		}
	}
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
