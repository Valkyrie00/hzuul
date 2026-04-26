package views

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

type KeyHint struct {
	Key   string
	Label string
	Style string // color tag, e.g. "#3884f4", "#e5c07b", "red"
}

type KeyBar struct {
	root        *tview.Flex
	hintsView   *tview.TextView
	statusView  *tview.TextView
	timeView    *tview.TextView
	globalHints []KeyHint
	viewStatus  string
	dlProgress  string
}

func NewKeyBar() *KeyBar {
	bg := ColorNavBg

	hints := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	hints.SetBackgroundColor(bg)

	status := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)
	status.SetBackgroundColor(bg)

	ts := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignRight)
	ts.SetBackgroundColor(bg)

	root := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(hints, 0, 1, false).
		AddItem(status, 0, 0, false).
		AddItem(ts, 22, 0, false)
	root.SetBackgroundColor(bg)

	return &KeyBar{
		root:       root,
		hintsView:  hints,
		statusView: status,
		timeView:   ts,
	}
}

func (kb *KeyBar) Root() *tview.Flex { return kb.root }

func (kb *KeyBar) SetGlobalHints(hints []KeyHint) {
	kb.globalHints = hints
}

// SetHints renders view-specific hints followed by a separator and the global hints.
func (kb *KeyBar) SetHints(hints []KeyHint) {
	kb.render("", hints, kb.globalHints)
}

// SetHintsWithFilter renders a filter prefix, then view + global hints.
func (kb *KeyBar) SetHintsWithFilter(filterText string, hints []KeyHint) {
	prefix := fmt.Sprintf("[#3884f4]/[-][white]%s[-]", filterText)
	kb.render(prefix, hints, kb.globalHints)
}

// SetMessage replaces all hints with a raw tview-formatted message string.
func (kb *KeyBar) SetMessage(msg string) {
	kb.hintsView.Clear()
	_, _ = fmt.Fprint(kb.hintsView, " "+msg)
}

func (kb *KeyBar) SetStatus(text string) {
	kb.viewStatus = text
	kb.renderStatus()
}

func (kb *KeyBar) SetDownloadProgress(text string) {
	kb.dlProgress = text
	kb.renderStatus()
}

func (kb *KeyBar) renderStatus() {
	kb.statusView.Clear()
	combined := kb.viewStatus
	if kb.dlProgress != "" {
		if combined != "" {
			combined += " [::d]│[-:-:-] " + kb.dlProgress
		} else {
			combined = kb.dlProgress
		}
	}
	if combined != "" {
		s := "[::d]│[-:-:-] " + combined + " [::d]│[-:-:-]"
		_, _ = fmt.Fprint(kb.statusView, s)
		kb.root.ResizeItem(kb.statusView, tview.TaggedStringWidth(combined)+6, 0)
	} else {
		kb.root.ResizeItem(kb.statusView, 0, 0)
	}
}

func (kb *KeyBar) ClearStatus() {
	kb.SetStatus("")
}

func (kb *KeyBar) SetTimestamp(ts string) {
	kb.timeView.Clear()
	_, _ = fmt.Fprintf(kb.timeView, "[::d]last update: %s [-:-:-]", ts)
}

func (kb *KeyBar) render(prefix string, viewHints []KeyHint, globalHints []KeyHint) {
	kb.hintsView.Clear()
	var b strings.Builder
	b.WriteString(" ")
	if prefix != "" {
		b.WriteString(prefix)
		b.WriteString("  ")
	}
	for i, h := range viewHints {
		if i > 0 {
			b.WriteString("  ")
		}
		kb.writeHint(&b, h)
	}
	if len(viewHints) > 0 && len(globalHints) > 0 {
		b.WriteString(" [::d]│[-:-:-] ")
	}
	for i, h := range globalHints {
		if i > 0 {
			b.WriteString("  ")
		}
		kb.writeHint(&b, h)
	}
	_, _ = fmt.Fprint(kb.hintsView, b.String())
}

func (kb *KeyBar) writeHint(b *strings.Builder, h KeyHint) {
	style := h.Style
	if style == "" {
		style = "#3884f4"
	}
	fmt.Fprintf(b, "[%s]%s[-:-:-][::d]:%s[-:-:-]", style, h.Key, h.Label)
}

var (
	HintBack         = KeyHint{"esc", "back", ""}
	HintScroll       = KeyHint{"↑↓", "scroll", ""}
	HintNavigate     = KeyHint{"↑↓", "navigate", ""}
	HintFilter       = KeyHint{"/", "filter", ""}
	HintHelp         = KeyHint{"?", "help", ""}
	HintTenant       = KeyHint{"t", "tenant", ""}
	HintRefresh      = KeyHint{"r", "refresh", ""}
	HintViews        = KeyHint{"1-9", "views", ""}
	HintQuit         = KeyHint{"q", "quit", ""}
	HintOpenWeb      = KeyHint{"o", "open web", ""}
	HintOpenChange   = KeyHint{"c", "open change", ""}
	HintBookmark     = KeyHint{"s", "toggle bookmark", ""}
	HintDownload     = KeyHint{"d", "download", ""}
	HintOpenLogs     = KeyHint{"l", "open logs", ""}
	HintAI           = KeyHint{"a", "AI analysis", "#e5c07b"}
	HintDequeue      = KeyHint{"x", "dequeue", ""}
	HintPromote      = KeyHint{"p", "promote", ""}
	HintReconnect    = KeyHint{"r", "reconnect", "yellow"}
	HintEnter        = KeyHint{"enter", "build detail", ""}
	HintExpand       = KeyHint{"enter", "expand/open", ""}
	HintDetail       = KeyHint{"enter", "buildset detail", ""}
	HintJobDetail    = KeyHint{"enter", "job detail", ""}
	HintRecent       = KeyHint{"enter", "recent builds", ""}
	HintDelete       = KeyHint{"d", "delete", ""}
	HintRemove       = KeyHint{"d", "remove", ""}
	HintCancel       = KeyHint{"x", "cancel", ""}
	HintOpenDir      = KeyHint{"o", "open dir", ""}
	HintOpenBuild    = KeyHint{"enter", "open build", ""}
	HintCreate       = KeyHint{"c", "create", ""}
	HintOpenBrowser  = KeyHint{"o", "open in browser", ""}
	HintOpenSource   = KeyHint{"o", "open source", ""}
	HintRecentBuilds = KeyHint{"e", "recent builds", ""}
	HintConfirmY     = KeyHint{"y", "confirm", "#48c78e"}
	HintConfirmN     = KeyHint{"n", "cancel", "#eb5757"}
)

func GlobalHints() []KeyHint {
	return []KeyHint{HintTenant, HintRefresh, HintViews, HintHelp, HintQuit}
}

func NewSpacer() *tview.Box {
	b := tview.NewBox()
	b.SetBackgroundColor(ColorBg)
	return b
}
