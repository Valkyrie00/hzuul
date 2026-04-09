package ai

import (
	"testing"

	"github.com/Valkyrie00/hzuul/internal/api"
)

func TestClassifyFailure(t *testing.T) {
	tests := []struct {
		name      string
		result    string
		tasks     []api.FailedTask
		playbooks []PlaybookSummary
		wantCat   string
		wantRetry bool
	}{
		{
			name:      "timed out is infra flake",
			result:    "TIMED_OUT",
			wantCat:   "INFRA_FLAKE",
			wantRetry: true,
		},
		{
			name:      "node failure is infra flake",
			result:    "NODE_FAILURE",
			wantCat:   "INFRA_FLAKE",
			wantRetry: true,
		},
		{
			name:      "retry limit is infra flake",
			result:    "RETRY_LIMIT",
			wantCat:   "INFRA_FLAKE",
			wantRetry: true,
		},
		{
			name:      "disk full is infra flake",
			result:    "DISK_FULL",
			wantCat:   "INFRA_FLAKE",
			wantRetry: true,
		},
		{
			name:      "merger failure is config error",
			result:    "MERGER_FAILURE",
			wantCat:   "CONFIG_ERROR",
			wantRetry: false,
		},
		{
			name:      "config error result",
			result:    "CONFIG_ERROR",
			wantCat:   "CONFIG_ERROR",
			wantRetry: false,
		},
		{
			name:   "post failure with run phase passed is infra flake",
			result: "POST_FAILURE",
			playbooks: []PlaybookSummary{
				{Phase: "run", Failed: false},
				{Phase: "post", Failed: true},
			},
			wantCat:   "INFRA_FLAKE",
			wantRetry: true,
		},
		{
			name:   "post failure with run phase also failed is real failure",
			result: "POST_FAILURE",
			tasks:  []api.FailedTask{{Task: "compile", Msg: "exit code 1"}},
			playbooks: []PlaybookSummary{
				{Phase: "run", Failed: true},
				{Phase: "post", Failed: true},
			},
			wantCat:   "REAL_FAILURE",
			wantRetry: false,
		},
		{
			name:      "failure with failed tasks is real failure",
			result:    "FAILURE",
			tasks:     []api.FailedTask{{Task: "run-tests", Msg: "assertion failed"}},
			wantCat:   "REAL_FAILURE",
			wantRetry: false,
		},
		{
			name:      "failure with no data is unknown",
			result:    "FAILURE",
			wantCat:   "UNKNOWN",
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyFailure(tt.result, tt.tasks, tt.playbooks)
			if got.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", got.Category, tt.wantCat)
			}
			if got.Retryable != tt.wantRetry {
				t.Errorf("Retryable = %v, want %v", got.Retryable, tt.wantRetry)
			}
		})
	}
}

func TestClassifyFailure_TruncatesLongMsg(t *testing.T) {
	longMsg := make([]byte, 200)
	for i := range longMsg {
		longMsg[i] = 'x'
	}
	c := ClassifyFailure("FAILURE", []api.FailedTask{{Task: "t", Msg: string(longMsg)}}, nil)
	if len(c.Reason) > 120 {
		t.Errorf("Reason not truncated: len=%d", len(c.Reason))
	}
}

func TestDetermineFailurePhase(t *testing.T) {
	tests := []struct {
		name      string
		playbooks []PlaybookSummary
		want      string
	}{
		{"no failures", []PlaybookSummary{{Phase: "run", Failed: false}}, ""},
		{"nil input", nil, ""},
		{"single run", []PlaybookSummary{{Phase: "run", Failed: true}}, "run"},
		{"pre normalizes to pre-run", []PlaybookSummary{{Phase: "pre", Failed: true}}, "pre-run"},
		{"setup normalizes to pre-run", []PlaybookSummary{{Phase: "setup", Failed: true}}, "pre-run"},
		{"post normalizes to post-run", []PlaybookSummary{{Phase: "post", Failed: true}}, "post-run"},
		{"cleanup normalizes to post-run", []PlaybookSummary{{Phase: "cleanup", Failed: true}}, "post-run"},
		{"empty phase is unknown", []PlaybookSummary{{Phase: "", Failed: true}}, "unknown"},
		{
			"mixed phases",
			[]PlaybookSummary{
				{Phase: "run", Failed: true},
				{Phase: "post", Failed: true},
			},
			"mixed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineFailurePhase(tt.playbooks); got != tt.want {
				t.Errorf("DetermineFailurePhase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlaybookSummaries(t *testing.T) {
	output := []api.PlaybookOutput{
		{
			Phase: "run",
			Stats: map[string]api.HostStats{
				"host1": {Ok: 5, Failures: 0},
			},
		},
		{
			Phase: "post",
			Stats: map[string]api.HostStats{
				"host1": {Ok: 2, Failures: 1},
			},
		},
	}

	got := PlaybookSummaries(output)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Phase != "run" || got[0].Failed != false {
		t.Errorf("got[0] = %+v, want run/false", got[0])
	}
	if got[1].Phase != "post" || got[1].Failed != true {
		t.Errorf("got[1] = %+v, want post/true", got[1])
	}
}

func TestPlaybookSummaries_Unreachable(t *testing.T) {
	output := []api.PlaybookOutput{
		{
			Phase: "run",
			Stats: map[string]api.HostStats{
				"host1": {Ok: 0, Unreachable: 1},
			},
		},
	}
	got := PlaybookSummaries(output)
	if !got[0].Failed {
		t.Error("expected Failed=true for unreachable host")
	}
}

func TestPlaybookSummaries_Empty(t *testing.T) {
	got := PlaybookSummaries(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestCategoryLabel(t *testing.T) {
	tests := []struct {
		cat  string
		want string
	}{
		{"INFRA_FLAKE", "[yellow]INFRA FLAKE[-]"},
		{"REAL_FAILURE", "[red]REAL FAILURE[-]"},
		{"CONFIG_ERROR", "[red]CONFIG ERROR[-]"},
		{"UNKNOWN", "[#78788c]UNKNOWN[-]"},
		{"OTHER", "[#78788c]UNKNOWN[-]"},
	}
	for _, tt := range tests {
		c := Classification{Category: tt.cat}
		if got := c.CategoryLabel(); got != tt.want {
			t.Errorf("CategoryLabel(%q) = %q, want %q", tt.cat, got, tt.want)
		}
	}
}

func TestRetryLabel(t *testing.T) {
	if got := (Classification{Retryable: true}).RetryLabel(); got != "[green]retryable[-]" {
		t.Errorf("RetryLabel(true) = %q", got)
	}
	if got := (Classification{Retryable: false}).RetryLabel(); got != "[red]not retryable[-]" {
		t.Errorf("RetryLabel(false) = %q", got)
	}
}
