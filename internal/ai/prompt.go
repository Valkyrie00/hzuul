package ai

import (
	"fmt"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/api"
)

const systemPrompt = `You are an expert Zuul CI/CD build failure analyst embedded in a terminal UI.

TASK: Diagnose the root cause of a build failure and tell the developer what to do next.

DOMAIN CONTEXT:
- Zuul runs Ansible playbooks in phases: pre-run (setup), run (actual job), post-run (log collection).
- A post-run failure with a passing run phase usually means log collection broke, not the job itself.
- Nested playbook failures (include_role/include_tasks) often wrap the real error — always trace to the innermost cause.
- Common infra flakes: SSH unreachable, DNS resolution, OOM kills, disk full, image pull failures, provisioning timeouts.
- The data you receive may be limited (only structured task output) or complete (with full log file snippets). Work with what you have.

RESPONSE FORMAT:
- Plain text only. No markdown, no headers, no code fences, no bullet symbols.
- Keep it short: 3-5 paragraphs max for the initial analysis.
- Structure: what failed, why it failed, what to do about it.
- If it looks like an infra flake, say so clearly and recommend a recheck.
- The user can ask follow-up questions, so don't try to cover everything upfront.`

func BuildAnalysisPrompt(build *api.Build, failedTasks []api.FailedTask, logContext []LogBlock) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Analyze this Zuul CI build failure:\n\n")
	fmt.Fprintf(&b, "Job: %s\n", build.JobName)
	fmt.Fprintf(&b, "Result: %s\n", build.Result)
	fmt.Fprintf(&b, "Project: %s\n", build.Ref.Project)
	fmt.Fprintf(&b, "Branch: %s\n", build.Ref.Branch)
	if build.Ref.Change != nil {
		fmt.Fprintf(&b, "Change: %v\n", build.Ref.Change)
	}
	fmt.Fprintf(&b, "Pipeline: %s\n", build.Pipeline)
	if build.Duration != nil {
		fmt.Fprintf(&b, "Duration: %v\n", build.Duration)
	}
	if build.ErrorDetail != "" {
		fmt.Fprintf(&b, "Error Detail: %s\n", build.ErrorDetail)
	}

	writeFailedTasks(&b, failedTasks)
	writeLogContext(&b, logContext, 4)

	fmt.Fprintf(&b, "\nProvide a concise analysis: root cause, category, and recommended action.")
	return b.String()
}

func BuildDirAnalysisPrompt(rec DirAnalysisInput, da *DirAnalysis) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Analyze this Zuul CI build failure using downloaded log files:\n\n")
	fmt.Fprintf(&b, "Job: %s\n", rec.JobName)
	fmt.Fprintf(&b, "Project: %s\n", rec.Project)
	if rec.DestDir != "" {
		fmt.Fprintf(&b, "Log directory: %s\n", rec.DestDir)
	}

	if len(da.AllFiles) > 0 {
		fmt.Fprintf(&b, "\n--- Complete file listing (%d files) ---\n", len(da.AllFiles))
		for _, f := range da.AllFiles {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}

	writeFailedTasks(&b, da.FailedTasks)
	writeLogContext(&b, da.LogContext, 6)

	if len(da.LogFiles) > 0 {
		fmt.Fprintf(&b, "\n--- Log File Snippets (tails from downloaded files) ---\n")
		for _, lf := range da.LogFiles {
			fmt.Fprintf(&b, "\n=== %s ===\n%s\n", lf.Path, lf.Content)
		}
	}

	fmt.Fprintf(&b, "\nThe user has the full log files at the directory shown above. ")
	fmt.Fprintf(&b, "When referencing specific files, use the relative paths from the listing. ")
	fmt.Fprintf(&b, "If the snippets aren't enough, suggest which file the user should look at for more detail.")
	fmt.Fprintf(&b, "\n\nProvide a concise analysis: root cause, category, and recommended action.")
	return b.String()
}

type DirAnalysisInput struct {
	JobName string
	Project string
	DestDir string
}

func GetSystemPrompt() string {
	return systemPrompt
}

func writeFailedTasks(b *strings.Builder, tasks []api.FailedTask) {
	if len(tasks) == 0 {
		return
	}
	fmt.Fprintf(b, "\n--- Failed Tasks ---\n")
	limit := len(tasks)
	if limit > 5 {
		limit = 5
	}
	for i, ft := range tasks[:limit] {
		fmt.Fprintf(b, "\n[%d] Task: %s\n", i+1, ft.Task)
		fmt.Fprintf(b, "    Host: %s\n", ft.Host)
		if ft.Action != "" {
			fmt.Fprintf(b, "    Action: %s\n", ft.Action)
		}
		if ft.Cmd != "" {
			fmt.Fprintf(b, "    Command: %s\n", truncateStr(ft.Cmd, 200))
		}
		if ft.Msg != "" {
			fmt.Fprintf(b, "    Message: %s\n", truncateStr(ft.Msg, 500))
		}
		if ft.Stderr != "" {
			fmt.Fprintf(b, "    Stderr: %s\n", truncateStr(ft.Stderr, 500))
		}
		if ft.Stdout != "" {
			stdout := ft.Stdout
			if len(stdout) > 1000 {
				stdout = stdout[len(stdout)-1000:]
			}
			fmt.Fprintf(b, "    Stdout (tail): %s\n", stdout)
		}
	}
	if len(tasks) > limit {
		fmt.Fprintf(b, "\n... and %d more failed tasks\n", len(tasks)-limit)
	}
}

func writeLogContext(b *strings.Builder, blocks []LogBlock, maxBlocks int) {
	if len(blocks) == 0 {
		return
	}
	fmt.Fprintf(b, "\n--- Log Context (fatal/FAILED lines with surrounding context) ---\n")
	for i, block := range blocks {
		if i >= maxBlocks {
			fmt.Fprintf(b, "\n... %d more context blocks\n", len(blocks)-i)
			break
		}
		fmt.Fprintf(b, "\n")
		for _, line := range block.Lines {
			marker := "  "
			if line.Match {
				marker = ">"
			}
			fmt.Fprintf(b, "%s L%d: %s\n", marker, line.N, line.Text)
		}
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
