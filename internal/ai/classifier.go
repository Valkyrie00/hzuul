package ai

import (
	"fmt"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/api"
)

type Classification struct {
	Category  string // INFRA_FLAKE, REAL_FAILURE, CONFIG_ERROR, UNKNOWN
	Reason    string
	Retryable bool
}

type PlaybookSummary struct {
	Phase  string
	Failed bool
}

func ClassifyFailure(result string, failedTasks []api.FailedTask, playbooks []PlaybookSummary) Classification {
	switch result {
	case "TIMED_OUT":
		return Classification{"INFRA_FLAKE", "Job timed out", true}
	case "NODE_FAILURE":
		return Classification{"INFRA_FLAKE", "Node failure", true}
	case "RETRY_LIMIT":
		return Classification{"INFRA_FLAKE", "Retry limit reached", true}
	case "DISK_FULL":
		return Classification{"INFRA_FLAKE", "Disk full", true}
	case "MERGER_FAILURE":
		return Classification{"CONFIG_ERROR", "Merge conflict or missing dependency", false}
	case "CONFIG_ERROR":
		return Classification{"CONFIG_ERROR", "Job configuration error", false}
	case "POST_FAILURE":
		runFailed := false
		for _, pb := range playbooks {
			if pb.Phase == "run" && pb.Failed {
				runFailed = true
				break
			}
		}
		if !runFailed {
			return Classification{"INFRA_FLAKE", "Post-run failed (run phase passed)", true}
		}
	}

	if len(failedTasks) > 0 {
		first := failedTasks[0]
		reason := fmt.Sprintf("Task '%s'", first.Task)
		if first.Msg != "" {
			msg := first.Msg
			if len(msg) > 80 {
				msg = msg[:80] + "..."
			}
			reason += ": " + msg
		}
		return Classification{"REAL_FAILURE", reason, false}
	}

	return Classification{"UNKNOWN", "No structured failure data", false}
}

func DetermineFailurePhase(playbooks []PlaybookSummary) string {
	phases := make(map[string]struct{})
	for _, pb := range playbooks {
		if !pb.Failed {
			continue
		}
		switch strings.ToLower(pb.Phase) {
		case "pre", "setup":
			phases["pre-run"] = struct{}{}
		case "run":
			phases["run"] = struct{}{}
		case "post", "cleanup":
			phases["post-run"] = struct{}{}
		case "":
			phases["unknown"] = struct{}{}
		default:
			phases[strings.ToLower(pb.Phase)] = struct{}{}
		}
	}
	if len(phases) == 0 {
		return ""
	}
	if len(phases) == 1 {
		for p := range phases {
			return p
		}
	}
	return "mixed"
}

func PlaybookSummaries(output []api.PlaybookOutput) []PlaybookSummary {
	summaries := make([]PlaybookSummary, 0, len(output))
	for _, pb := range output {
		hasFail := false
		for _, s := range pb.Stats {
			if s.Failures > 0 || s.Unreachable > 0 {
				hasFail = true
				break
			}
		}
		summaries = append(summaries, PlaybookSummary{
			Phase:  pb.Phase,
			Failed: hasFail,
		})
	}
	return summaries
}

func (c Classification) CategoryLabel() string {
	switch c.Category {
	case "INFRA_FLAKE":
		return "[yellow]INFRA FLAKE[-]"
	case "REAL_FAILURE":
		return "[red]REAL FAILURE[-]"
	case "CONFIG_ERROR":
		return "[red]CONFIG ERROR[-]"
	default:
		return "[#78788c]UNKNOWN[-]"
	}
}

func (c Classification) RetryLabel() string {
	if c.Retryable {
		return "[green]retryable[-]"
	}
	return "[red]not retryable[-]"
}
