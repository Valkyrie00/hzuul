package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/vcastell/hzuul/internal/api"
)

type JobsView struct {
	root         *tview.Flex
	table        *tview.Table
	detailHeader *tview.TextView
	detailView   *tview.TextView
	buildTable   *tview.Table
	logView      *BuildLogView
	pages      *tview.Pages
	app        *tview.Application
	client     *api.Client
	jobs       []api.Job
	indexMap    []int
	filter     string

	buildBuilds []api.Build
	currentJob  string
	sourceURL   string // URL to the file where the job is defined
	page        string // "table", "detail", "builds", "buildlog"
}

func NewJobsView(app *tview.Application) *JobsView {
	bg := tcell.NewRGBColor(24, 24, 32)
	navBg := tcell.NewRGBColor(32, 32, 44)

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	table.SetBackgroundColor(bg)
	table.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	keys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	keys.SetBackgroundColor(navBg)
	fmt.Fprint(keys, " [blue]enter[-:-:-][::d]:job detail[-:-:-]  [blue]o[-:-:-][::d]:open in browser[-:-:-]  [blue]↑↓[-:-:-][::d]:navigate[-:-:-]")

	tableWithKeys := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(keys, 1, 0, false)
	tableWithKeys.SetBackgroundColor(bg)

	detailHeader := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	detailHeader.SetBackgroundColor(navBg)

	dimColor := tcell.NewRGBColor(70, 70, 90)
	detailSep := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	detailSep.SetBackgroundColor(bg)
	detailSep.SetTextColor(dimColor)
	fmt.Fprint(detailSep, "  ──────────────────────────────────────")

	detailView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	detailView.SetBackgroundColor(bg)
	detailView.SetBorderPadding(0, 1, 2, 2)

	detailKeys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	detailKeys.SetBackgroundColor(navBg)
	fmt.Fprint(detailKeys, " [blue]b[-:-:-][::d]:recent builds[-:-:-]  [blue]o[-:-:-][::d]:open source[-:-:-]  [blue]esc[-:-:-][::d]:back[-:-:-]  [blue]↑↓[-:-:-][::d]:scroll[-:-:-]")

	detailPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(detailHeader, 1, 0, false).
		AddItem(detailSep, 1, 0, false).
		AddItem(detailView, 0, 1, true).
		AddItem(detailKeys, 1, 0, false)
	detailPage.SetBackgroundColor(bg)

	buildTable := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0)
	buildTable.SetBackgroundColor(bg)
	buildTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 30, 42)).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))

	buildHeader := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	buildHeader.SetBackgroundColor(navBg)

	buildKeys := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	buildKeys.SetBackgroundColor(navBg)
	fmt.Fprint(buildKeys, " [blue]enter[-:-:-][::d]:build detail[-:-:-]  [blue]esc[-:-:-][::d]:back[-:-:-]  [blue]↑↓[-:-:-][::d]:navigate[-:-:-]")

	buildPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(buildHeader, 1, 0, false).
		AddItem(buildTable, 0, 1, true).
		AddItem(buildKeys, 1, 0, false)
	buildPage.SetBackgroundColor(bg)

	logView := NewBuildLogView(app)

	pages := tview.NewPages().
		AddPage("table", tableWithKeys, true, true).
		AddPage("detail", detailPage, true, false).
		AddPage("builds", buildPage, true, false).
		AddPage("buildlog", logView.Root(), true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true)
	root.SetBackgroundColor(bg)

	v := &JobsView{
		root:         root,
		table:        table,
		detailHeader: detailHeader,
		detailView:   detailView,
		buildTable:   buildTable,
		logView:    logView,
		pages:      pages,
		app:        app,
		page:       "table",
	}

	table.SetSelectedFunc(func(row, _ int) {
		idx := v.jobIndex(row)
		if idx < 0 || v.client == nil {
			return
		}
		j := v.jobs[idx]
		v.showJobDetail(j)
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'o' {
			row, _ := table.GetSelection()
			idx := v.jobIndex(row)
			if idx >= 0 && v.client != nil {
				openURL(v.client.JobURL(v.jobs[idx].Name))
			}
			return nil
		}
		return event
	})

	detailPage.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			v.pages.SwitchToPage("table")
			v.page = "table"
			return nil
		}
		if event.Rune() == 'b' && v.currentJob != "" {
			v.showJobBuilds(v.currentJob, buildHeader)
			return nil
		}
		if event.Rune() == 'o' {
			if v.sourceURL != "" {
				openURL(v.sourceURL)
			} else if v.currentJob != "" && v.client != nil {
				openURL(v.client.JobURL(v.currentJob))
			}
			return nil
		}
		return event
	})

	buildTable.SetSelectedFunc(func(row, _ int) {
		bi := row - 1
		if bi < 0 || bi >= len(v.buildBuilds) {
			return
		}
		build := v.buildBuilds[bi]
		v.logView.ShowStaticLog(v.client, &build)
		v.pages.SwitchToPage("buildlog")
		v.page = "buildlog"
	})

	buildTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			v.pages.SwitchToPage("detail")
			v.page = "detail"
			return nil
		}
		return event
	})

	logView.Root().(*tview.Flex).SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyEsc {
			v.logView.Stop()
			v.pages.SwitchToPage("builds")
			v.page = "builds"
			return nil
		}
		if event.Rune() == 'o' && v.logView.openURL != "" {
			openURL(v.logView.openURL)
			return nil
		}
		if event.Rune() == 'l' && v.logView.logURL != "" {
			openURL(v.logView.logURL)
			return nil
		}
		return event
	})

	return v
}

func (v *JobsView) Root() tview.Primitive { return v.root }

func (v *JobsView) SetFilter(term string) {
	v.filter = term
	v.renderTable()
	v.table.Select(1, 0)
}

func (v *JobsView) jobIndex(row int) int {
	ri := row - 1
	if ri < 0 || ri >= len(v.indexMap) {
		return -1
	}
	return v.indexMap[ri]
}

func (v *JobsView) Load(client *api.Client) {
	v.client = client
	if v.page != "table" {
		return
	}
	go func() {
		jobs, err := client.GetJobs()
		v.app.QueueUpdateDraw(func() {
			v.table.Clear()
			setTableHeader(v.table, "Name", "Description")
			if err != nil {
				v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)))
				return
			}
			v.jobs = jobs
			v.renderTable()
			v.table.Select(1, 0)
			v.table.ScrollToBeginning()
		})
	}()
}

func (v *JobsView) renderTable() {
	v.table.Clear()
	setTableHeader(v.table, "Name", "Description")
	muted := tcell.NewRGBColor(120, 120, 140)
	v.indexMap = nil
	row := 1
	for i, j := range v.jobs {
		tags := strings.Join(j.Tags, ", ")
		if !rowMatchesFilter(v.filter, j.Name, j.Description, tags) {
			continue
		}
		v.indexMap = append(v.indexMap, i)
		v.table.SetCell(row, 0, tview.NewTableCell(" "+j.Name).SetTextColor(tcell.ColorWhite))
		v.table.SetCell(row, 1, tview.NewTableCell(" "+j.Description).SetTextColor(muted).SetExpansion(1))
		row++
	}
	if row == 1 && v.filter != "" {
		v.table.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [::d]No matches for '%s'[-]", v.filter)).SetSelectable(false))
	}
}

func formatValue(val any) string {
	return formatValueIndent(val, 0)
}

func formatValueIndent(val any, depth int) string {
	indent := strings.Repeat("      ", depth)
	childIndent := strings.Repeat("      ", depth+1)

	switch v := val.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%v", v)
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%g", v)
	case map[string]any:
		if len(v) <= 3 {
			parts := make([]string, 0, len(v))
			simple := true
			for k, inner := range v {
				s := formatValue(inner)
				if len(s) > 40 {
					simple = false
					break
				}
				parts = append(parts, fmt.Sprintf("%s: %s", k, s))
			}
			if simple {
				return "{" + strings.Join(parts, ", ") + "}"
			}
		}
		var b strings.Builder
		b.WriteString("{\n")
		for k, inner := range v {
			fmt.Fprintf(&b, "%s%s: %s\n", childIndent, k, formatValueIndent(inner, depth+1))
		}
		b.WriteString(indent + "}")
		return b.String()
	case []any:
		if len(v) <= 3 {
			parts := make([]string, 0, len(v))
			totalLen := 0
			for _, item := range v {
				s := formatValue(item)
				totalLen += len(s)
				parts = append(parts, s)
			}
			if totalLen < 80 {
				return "[" + strings.Join(parts, ", ") + "]"
			}
		}
		var b strings.Builder
		b.WriteString("[\n")
		for _, item := range v {
			fmt.Fprintf(&b, "%s- %s\n", childIndent, formatValueIndent(item, depth+1))
		}
		b.WriteString(indent + "]")
		return b.String()
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func buildSourceURL(src map[string]any, variant map[string]any) string {
	branch, _ := src["branch"].(string)
	filePath, _ := src["path"].(string)
	if filePath == "" {
		return ""
	}

	canonical, _ := src["project_canonical_name"].(string)
	if canonical == "" {
		shortName, _ := src["project"].(string)
		if shortName != "" && variant != nil {
			if roles, ok := variant["roles"].([]any); ok {
				for _, r := range roles {
					if rm, ok := r.(map[string]any); ok {
						cn, _ := rm["project_canonical_name"].(string)
						if cn != "" && strings.HasSuffix(cn, "/"+shortName) {
							canonical = cn
							break
						}
					}
				}
			}
		}
	}

	if canonical == "" {
		return ""
	}

	parts := strings.SplitN(canonical, "/", 2)
	if len(parts) != 2 || !strings.Contains(parts[0], ".") {
		return ""
	}
	host := parts[0]
	project := parts[1]
	if branch == "" {
		branch = "main"
	}
	return fmt.Sprintf("https://%s/%s/-/blob/%s/%s", host, project, branch, filePath)
}

func (v *JobsView) showJobDetail(j api.Job) {
	v.currentJob = j.Name
	v.sourceURL = ""

	v.detailHeader.Clear()
	fmt.Fprintf(v.detailHeader, " [bold]Job Detail[-:-:-] │ [blue]%s[-]", j.Name)

	v.detailView.Clear()
	if j.Description != "" {
		fmt.Fprintf(v.detailView, "\n[bold]Description[-:-:-]\n  %s\n", j.Description)
	}

	if len(j.Tags) > 0 {
		fmt.Fprintf(v.detailView, "\n[bold]Tags[-:-:-]         %s\n", strings.Join(j.Tags, ", "))
	}

	fmt.Fprintf(v.detailView, "\n[::d]Loading full details...[-:-:-]")

	v.pages.SwitchToPage("detail")
	v.page = "detail"

	go func() {
		details, err := v.client.GetJob(j.Name)
		v.app.QueueUpdateDraw(func() {
			v.detailView.Clear()

			if j.Description != "" {
				fmt.Fprintf(v.detailView, "\n[bold]Description[-:-:-]\n  %s\n", j.Description)
			}

			thickLine := "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

			if err != nil {
				fmt.Fprintf(v.detailView, "\n[red]Error loading details: %v[-]\n", err)
				return
			}

			if len(details) == 0 {
				fmt.Fprintf(v.detailView, "\n[yellow]No variant details available[-]\n")
				return
			}

			parentName := ""
			for vi, variant := range details {
				fmt.Fprintf(v.detailView, "\n[blue]%s[-]\n", thickLine)

				branchLabel := "all branches"
				if branches, ok := variant["branches"].([]any); ok && len(branches) > 0 {
					parts := make([]string, 0, len(branches))
					for _, b := range branches {
						parts = append(parts, fmt.Sprintf("%v", b))
					}
					branchLabel = strings.Join(parts, ", ")
				}
				fmt.Fprintf(v.detailView, "[bold]Variant %d[-:-:-]  [::d](%s)[-:-:-]\n", vi+1, branchLabel)
				fmt.Fprintf(v.detailView, "[blue]%s[-]\n\n", thickLine)

				if p, ok := variant["parent"].(string); ok && p != "" {
					if parentName == "" {
						parentName = p
					}
					fmt.Fprintf(v.detailView, "  [bold]Parent[-:-:-]          %s\n", p)
				}
				if voting, ok := variant["voting"].(bool); ok {
					flag := "[green]Voting[-]"
					if !voting {
						flag = "[yellow]Non-voting[-]"
					}
					fmt.Fprintf(v.detailView, "  [bold]Voting[-:-:-]          %s\n", flag)
				}
				if attempts, ok := variant["attempts"].(float64); ok {
					fmt.Fprintf(v.detailView, "  [bold]Retry attempts[-:-:-]  %.0f\n", attempts)
				}
				if timeout, ok := variant["timeout"].(float64); ok {
					fmt.Fprintf(v.detailView, "  [bold]Timeout[-:-:-]         %s\n", formatBuildDuration(timeout))
				}
				if nodeset, ok := variant["nodeset"].(map[string]any); ok {
					if nodes, ok := nodeset["nodes"].([]any); ok && len(nodes) > 0 {
						fmt.Fprintf(v.detailView, "  [bold]Nodes[-:-:-]           ")
						for ni, n := range nodes {
							if nm, ok := n.(map[string]any); ok {
								name, _ := nm["name"].(string)
								label, _ := nm["label"].(string)
								if ni > 0 {
									fmt.Fprint(v.detailView, ", ")
								}
								fmt.Fprintf(v.detailView, "%s [::d](%s)[-:-:-]", name, label)
							}
						}
						fmt.Fprintln(v.detailView)
					}
				}

				if sem, ok := variant["semaphores"].([]any); ok && len(sem) > 0 {
					parts := make([]string, 0)
					for _, s := range sem {
						if sm, ok := s.(map[string]any); ok {
							if name, ok := sm["name"].(string); ok {
								parts = append(parts, name)
							}
						}
					}
					if len(parts) > 0 {
						fmt.Fprintf(v.detailView, "  [bold]Semaphores[-:-:-]      %s\n", strings.Join(parts, ", "))
					}
				}

				if src, ok := variant["source_context"].(map[string]any); ok {
					project, _ := src["project"].(string)
					branch, _ := src["branch"].(string)
					path, _ := src["path"].(string)
					if project != "" {
						loc := project
						if branch != "" {
							loc += fmt.Sprintf(" (%s)", branch)
						}
						if path != "" {
							loc += ": " + path
						}
						fmt.Fprintf(v.detailView, "  [bold]Defined at[-:-:-]      %s\n", loc)
					}
					if v.sourceURL == "" {
						v.sourceURL = buildSourceURL(src, variant)
					}
				}

				if roles, ok := variant["roles"].([]any); ok && len(roles) > 0 {
					fmt.Fprintf(v.detailView, "\n  [bold]Roles[-:-:-]\n")
					for _, r := range roles {
						if rm, ok := r.(map[string]any); ok {
							project, _ := rm["project_canonical_name"].(string)
							if project == "" {
								project, _ = rm["project_name"].(string)
							}
							if project != "" {
								fmt.Fprintf(v.detailView, "    [::d]•[-:-:-] %s\n", project)
							}
						}
					}
				}

				if vars, ok := variant["variables"].(map[string]any); ok && len(vars) > 0 {
					fmt.Fprintf(v.detailView, "\n  [bold]Variables[-:-:-]  [::d](%d items)[-:-:-]\n", len(vars))
					count := 0
					for k, val := range vars {
						if count >= 15 {
							fmt.Fprintf(v.detailView, "    [::d]... and %d more[-:-:-]\n", len(vars)-15)
							break
						}
						s := formatValue(val)
						fmt.Fprintf(v.detailView, "    [::d]%s[-:-:-] = %s\n", k, s)
						count++
					}
				}
			}

			// Fetch parent chain from the first variant's parent
			if parentName != "" {
				fmt.Fprintf(v.detailView, "\n[blue]%s[-]\n", thickLine)
				fmt.Fprintf(v.detailView, "[bold]Inheritance[-:-:-]\n")
				fmt.Fprintf(v.detailView, "[blue]%s[-]\n\n", thickLine)
				fmt.Fprintf(v.detailView, "  [::d]Loading parent chain...[-:-:-]")
				go v.fetchParentChain(j.Name, parentName)
			}
		})
	}()
}

func (v *JobsView) fetchParentChain(jobName, firstParent string) {
	chain := []string{jobName}
	current := firstParent
	for current != "" && len(chain) < 20 {
		chain = append(chain, current)
		variants, err := v.client.GetJob(current)
		if err != nil || len(variants) == 0 {
			break
		}
		parent, _ := variants[0]["parent"].(string)
		if parent == current {
			break
		}
		current = parent
	}

	v.app.QueueUpdateDraw(func() {
		text := v.detailView.GetText(false)
		text = strings.Replace(text, "  [::d]Loading parent chain...[-:-:-]", "", 1)
		v.detailView.Clear()
		fmt.Fprint(v.detailView, text)

		for i := len(chain) - 1; i >= 0; i-- {
			depth := len(chain) - 1 - i
			indent := strings.Repeat("   ", depth)
			if i == 0 {
				fmt.Fprintf(v.detailView, "  %s└─ [bold][blue]%s[-][-:-:-]  [::d](current)[-:-:-]\n", indent, chain[i])
			} else if i == len(chain)-1 {
				fmt.Fprintf(v.detailView, "  %s\n", chain[i])
			} else {
				fmt.Fprintf(v.detailView, "  %s└─ %s\n", indent, chain[i])
			}
		}
	})
}

func (v *JobsView) showJobBuilds(jobName string, header *tview.TextView) {
	v.buildTable.Clear()
	setTableHeader(v.buildTable, "Project", "Branch", "Pipeline", "Change", "Result", "Duration", "Start")
	v.buildTable.SetCell(1, 0, tview.NewTableCell(" [::d]Loading...[-:-:-]").SetSelectable(false))

	header.Clear()
	fmt.Fprintf(header, " [bold]%s[-:-:-]  [::d]recent builds[-:-:-]", jobName)

	v.pages.SwitchToPage("builds")
	v.page = "builds"

	go func() {
		builds, err := v.client.GetBuilds(&api.BuildFilter{
			JobName: jobName,
			Limit:   30,
		})
		v.app.QueueUpdateDraw(func() {
			v.buildTable.Clear()
			setTableHeader(v.buildTable, "Project", "Branch", "Pipeline", "Change", "Result", "Duration", "Start")
			if err != nil {
				v.buildTable.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [red]Error: %v[-]", err)).SetSelectable(false))
				return
			}
			if len(builds) == 0 {
				v.buildTable.SetCell(1, 0, tview.NewTableCell(fmt.Sprintf(" [yellow]No builds found for %s[-]", jobName)).SetSelectable(false))
				return
			}
			v.buildBuilds = builds
			muted := tcell.NewRGBColor(120, 120, 140)
			dim := tcell.NewRGBColor(90, 90, 110)
			for i, b := range builds {
				row := i + 1
				change := ""
				if b.Ref.Change != nil {
					change = fmt.Sprintf("%v", b.Ref.Change)
				}
				rc := resultColor(b.Result)
				v.buildTable.SetCell(row, 0, tview.NewTableCell(" "+resultIcon(b.Result)+" "+b.Ref.Project).SetTextColor(rc))
				v.buildTable.SetCell(row, 1, tview.NewTableCell(" "+b.Ref.Branch).SetTextColor(muted))
				v.buildTable.SetCell(row, 2, tview.NewTableCell(" "+b.Pipeline).SetTextColor(muted))
				v.buildTable.SetCell(row, 3, tview.NewTableCell(" "+change).SetTextColor(muted))
				v.buildTable.SetCell(row, 4, resultCell(b.Result))
				v.buildTable.SetCell(row, 5, tview.NewTableCell(" "+formatBuildDuration(b.Duration)).SetTextColor(muted))
				v.buildTable.SetCell(row, 6, tview.NewTableCell(" "+b.StartTime).SetTextColor(dim))
			}
			v.buildTable.Select(1, 0)
		})
	}()
}
