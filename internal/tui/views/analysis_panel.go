package views

import (
	"fmt"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/ai"
	"github.com/Valkyrie00/hzuul/internal/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type AnalysisMode string

const (
	AnalysisBasic AnalysisMode = "basic"
	AnalysisFull  AnalysisMode = "full"
)

type AnalysisPanel struct {
	layout  *tview.Flex
	header  *tview.TextView
	content *tview.TextView
	input   *tview.InputField
	app     *tview.Application

	active      bool
	analyzer    *ai.Analyzer
	history     []string
	streaming   bool
	inputActive bool
	mode        AnalysisMode
	aiCfg       config.AIConfig

	onKeysChanged func()
	onExit        func()
}

func NewAnalysisPanel(app *tview.Application, aiCfg config.AIConfig) *AnalysisPanel {
	bg := ColorBg

	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	header.SetBackgroundColor(bg)
	header.SetBorderPadding(0, 0, 2, 0)

	separator := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	separator.SetBackgroundColor(bg)
	separator.SetTextColor(ColorSep)
	_, _ = fmt.Fprint(separator, "  ──────────────────────────────────────")

	content := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() { app.Draw() })
	content.SetBackgroundColor(bg)
	content.SetBorderPadding(0, 0, 2, 2)

	input := tview.NewInputField()
	input.SetBackgroundColor(ColorNavBg)
	input.SetFieldBackgroundColor(ColorNavBg)
	input.SetFieldTextColor(tcell.ColorWhite)
	input.SetLabelColor(tcell.ColorGoldenrod)
	input.SetLabel(" Ask: ")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 1, 0, false).
		AddItem(separator, 1, 0, false).
		AddItem(content, 0, 1, true)
	layout.SetBackgroundColor(bg)

	return &AnalysisPanel{
		layout:  layout,
		header:  header,
		content: content,
		input:   input,
		app:     app,
		aiCfg:   aiCfg,
	}
}

func (p *AnalysisPanel) Root() *tview.Flex        { return p.layout }
func (p *AnalysisPanel) IsActive() bool           { return p.active }
func (p *AnalysisPanel) IsStreaming() bool        { return p.streaming }
func (p *AnalysisPanel) IsInputActive() bool      { return p.inputActive }
func (p *AnalysisPanel) HasAnalyzer() bool        { return p.analyzer != nil }
func (p *AnalysisPanel) Content() *tview.TextView { return p.content }

func (p *AnalysisPanel) SetOnKeysChanged(fn func()) { p.onKeysChanged = fn }
func (p *AnalysisPanel) SetOnExit(fn func())        { p.onExit = fn }

func (p *AnalysisPanel) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !p.active {
		return event
	}
	if p.inputActive {
		return event
	}
	if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
		p.Exit()
		return nil
	}
	if event.Rune() == 'a' && p.analyzer != nil && !p.streaming {
		p.showInput()
		return nil
	}
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		return event
	}
	return nil
}

func (p *AnalysisPanel) KeyHints() []KeyHint {
	hints := []KeyHint{{"esc", "back", ""}}
	if p.analyzer != nil && !p.streaming {
		hints = append(hints, KeyHint{"a", "ask", "#e5c07b"})
	}
	hints = append(hints, KeyHint{"↑↓", "scroll", ""})
	return hints
}

func (p *AnalysisPanel) UpdateKeys() {
	if p.onKeysChanged != nil {
		p.onKeysChanged()
	}
}

func (p *AnalysisPanel) Start(mode AnalysisMode, jobName, project string) {
	p.active = true
	p.mode = mode
	p.history = nil
	p.analyzer = ai.NewAnalyzer(p.aiCfg)

	p.header.Clear()
	modeLabel := "Quick Analysis"
	if mode == AnalysisFull {
		modeLabel = "Full Analysis"
	}
	_, _ = fmt.Fprintf(p.header, " [bold][#e5c07b]%s[-] │ [#3884f4]%s[-] │ %s", modeLabel, jobName, project)

	p.content.Clear()
	p.content.ScrollToBeginning()
	p.UpdateKeys()
}

func (p *AnalysisPanel) WriteClassification(classification ai.Classification, phase string) {
	parts := []string{classification.CategoryLabel(), classification.RetryLabel()}
	if phase != "" {
		parts = append(parts, phase+" phase")
	}
	if classification.Reason != "" {
		reason := classification.Reason
		if len(reason) > 80 {
			reason = reason[:80] + "..."
		}
		parts = append(parts, reason)
	}
	_, _ = fmt.Fprintf(p.content, "[#e5c07b]⚡[-] %s\n", strings.Join(parts, " · "))
}

func (p *AnalysisPanel) WriteNoAI() {
	w := p.content
	_, _ = fmt.Fprintf(w, "\n[#e5c07b]  ──────────────────────────────────────[-]\n\n")
	_, _ = fmt.Fprintf(w, "  [bold]AI Analysis not configured[-]\n\n")
	_, _ = fmt.Fprintf(w, "  [::d]Enable AI-powered deep analysis by adding to [white]~/.hzuul/config.yaml[-:-:-][::d]:[-:-:-]\n\n")
	_, _ = fmt.Fprintf(w, "  [::d]  # Anthropic API (direct)[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]  ai:[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]    provider: anthropic[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]    anthropic_api_key: sk-ant-...[-:-:-]\n\n")
	_, _ = fmt.Fprintf(w, "  [::d]  # Google Vertex AI (Claude via GCP)[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]  ai:[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]    provider: vertex[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]    vertex_project_id: my-project[-:-:-]\n")
	_, _ = fmt.Fprintf(w, "  [::d]    vertex_region: us-east5[-:-:-]\n\n")
	_, _ = fmt.Fprintf(w, "  [::d]  ... and more (Gemini, Gemini Vertex AI, etc.)[-:-:-]\n\n")
	_, _ = fmt.Fprintf(w, "  [::d]See the HZUUL README for all supported providers and setup details.[-:-:-]\n")
}

func (p *AnalysisPanel) StartAI(systemPrompt, userPrompt string) {
	if p.analyzer == nil {
		p.WriteNoAI()
		return
	}

	p.writeAIHeader()
	p.history = append(p.history, "User: "+userPrompt)
	p.runAI(systemPrompt, userPrompt)
}

func (p *AnalysisPanel) writeAIHeader() {
	thickLine := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	_, _ = fmt.Fprintf(p.content, "\n[#e5c07b]%s[-]\n", thickLine)
	_, _ = fmt.Fprintf(p.content, "[bold][#e5c07b]AI Analysis[-] [#e5c07b](%s · %s)[-]", p.analyzer.ProviderName(), p.analyzer.ModelName())
	if p.mode == AnalysisBasic {
		_, _ = fmt.Fprintf(p.content, " [::d]— download logs for full analysis[-:-:-]")
	}
	_, _ = fmt.Fprint(p.content, "\n")
	_, _ = fmt.Fprintf(p.content, "[#e5c07b]%s[-]\n", thickLine)
}

func (p *AnalysisPanel) runAI(systemPrompt, userPrompt string) {
	p.streaming = true
	p.UpdateKeys()
	_, _ = fmt.Fprint(p.content, "[::d]Analyzing...[-:-:-]")

	var responseBuf strings.Builder
	firstChunk := true

	p.analyzer.Analyze(systemPrompt, userPrompt,
		func(chunk string) {
			responseBuf.WriteString(chunk)
			p.app.QueueUpdateDraw(func() {
				if !p.active {
					return
				}
				if firstChunk {
					firstChunk = false
					_, _ = fmt.Fprint(p.content, "\n\n")
				}
				_, _ = fmt.Fprint(p.content, chunk)
				p.content.ScrollToEnd()
			})
		},
		func(err error) {
			p.app.QueueUpdateDraw(func() {
				p.streaming = false
				if !p.active {
					return
				}
				if err != nil {
					_, _ = fmt.Fprintf(p.content, "\n\n[red]  Error: %v[-]\n", err)
				} else {
					p.history = append(p.history, "Assistant: "+responseBuf.String())
					_, _ = fmt.Fprint(p.content, "\n")
				}
				p.UpdateKeys()
			})
		},
	)
}

func (p *AnalysisPanel) showInput() {
	p.inputActive = true
	p.input.SetText("")

	p.input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			question := strings.TrimSpace(p.input.GetText())
			if question == "" {
				return
			}
			p.hideInput()
			p.askFollowUp(question)
		case tcell.KeyEsc:
			p.hideInput()
		}
	})

	p.layout.AddItem(p.input, 1, 0, true)
	p.app.SetFocus(p.input)
	p.UpdateKeys()
}

func (p *AnalysisPanel) hideInput() {
	p.inputActive = false
	p.layout.RemoveItem(p.input)
	p.app.SetFocus(p.content)
	p.UpdateKeys()
}

func (p *AnalysisPanel) askFollowUp(question string) {
	thinLine := "────────────────────────────────────────────────────────────────────────────────"
	_, _ = fmt.Fprintf(p.content, "\n\n[#e5c07b]%s[-]\n", thinLine)
	_, _ = fmt.Fprintf(p.content, "[bold][#e5c07b]> %s[-]\n", question)
	_, _ = fmt.Fprintf(p.content, "[#e5c07b]%s[-]\n", thinLine)

	var fullPrompt strings.Builder
	for _, h := range p.history {
		_, _ = fmt.Fprintf(&fullPrompt, "%s\n\n", h)
	}
	_, _ = fmt.Fprintf(&fullPrompt, "User follow-up question: %s", question)

	p.history = append(p.history, "User: "+question)
	p.runAI(ai.GetSystemPrompt(), fullPrompt.String())
}

func (p *AnalysisPanel) Exit() {
	p.active = false
	p.analyzer = nil
	p.history = nil
	p.streaming = false
	p.inputActive = false
	p.content.Clear()

	if p.onExit != nil {
		p.onExit()
	}
}
