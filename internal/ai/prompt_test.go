package ai

import (
	"strings"
	"testing"

	"github.com/Valkyrie00/hzuul/internal/api"
)

func TestGetSystemPrompt(t *testing.T) {
	p := GetSystemPrompt()
	if p == "" {
		t.Fatal("system prompt should not be empty")
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("short", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := truncateStr("hello world", 5); got != "hello..." {
		t.Errorf("got %q", got)
	}
	if got := truncateStr("exact", 5); got != "exact" {
		t.Errorf("got %q", got)
	}
}

func TestWriteFailedTasks_Empty(t *testing.T) {
	var b strings.Builder
	writeFailedTasks(&b, nil)
	if b.Len() != 0 {
		t.Error("expected no output for nil tasks")
	}
}

func TestWriteFailedTasks(t *testing.T) {
	var b strings.Builder
	tasks := []api.FailedTask{
		{Task: "compile", Host: "h1", Action: "shell", Cmd: "make", Msg: "exit 1", Stderr: "err", Stdout: "out"},
	}
	writeFailedTasks(&b, tasks)
	out := b.String()
	for _, want := range []string{"compile", "h1", "shell", "make", "exit 1", "err", "out"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output", want)
		}
	}
}

func TestWriteFailedTasks_LimitsTo5(t *testing.T) {
	var b strings.Builder
	tasks := make([]api.FailedTask, 8)
	for i := range tasks {
		tasks[i] = api.FailedTask{Task: "t", Host: "h"}
	}
	writeFailedTasks(&b, tasks)
	if !strings.Contains(b.String(), "3 more failed tasks") {
		t.Error("expected overflow message")
	}
}

func TestWriteFailedTasks_TruncatesLongStdout(t *testing.T) {
	var b strings.Builder
	long := strings.Repeat("x", 2000)
	writeFailedTasks(&b, []api.FailedTask{{Task: "t", Host: "h", Stdout: long}})
	if strings.Contains(b.String(), long) {
		t.Error("expected stdout to be truncated")
	}
}

func TestWriteLogContext_Empty(t *testing.T) {
	var b strings.Builder
	writeLogContext(&b, nil, 5)
	if b.Len() != 0 {
		t.Error("expected no output for nil blocks")
	}
}

func TestWriteLogContext(t *testing.T) {
	var b strings.Builder
	blocks := []LogBlock{
		{Lines: []LogLine{
			{N: 1, Text: "ok", Match: false},
			{N: 2, Text: "fatal: boom", Match: true},
		}},
	}
	writeLogContext(&b, blocks, 5)
	out := b.String()
	if !strings.Contains(out, "> L2:") {
		t.Error("expected match marker for line 2")
	}
	if !strings.Contains(out, "  L1:") {
		t.Error("expected context marker for line 1")
	}
}

func TestWriteLogContext_LimitsBlocks(t *testing.T) {
	var b strings.Builder
	blocks := make([]LogBlock, 5)
	for i := range blocks {
		blocks[i] = LogBlock{Lines: []LogLine{{N: i, Text: "x"}}}
	}
	writeLogContext(&b, blocks, 2)
	if !strings.Contains(b.String(), "3 more context blocks") {
		t.Error("expected overflow message")
	}
}

func TestBuildAnalysisPrompt(t *testing.T) {
	build := &api.Build{
		JobName:  "tox-py312",
		Pipeline: "check",
		Result:   "FAILURE",
		Ref:      api.BuildRef{Project: "openstack/nova", Branch: "master"},
	}
	tasks := []api.FailedTask{{Task: "run-tests", Host: "h1", Msg: "failed"}}
	blocks := []LogBlock{{Lines: []LogLine{{N: 1, Text: "fatal: error", Match: true}}}}

	prompt := BuildAnalysisPrompt(build, tasks, blocks)
	for _, want := range []string{"tox-py312", "openstack/nova", "run-tests", "fatal: error"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("missing %q in prompt", want)
		}
	}
}

func TestBuildDirAnalysisPrompt(t *testing.T) {
	input := DirAnalysisInput{JobName: "deploy", Project: "myproject"}
	da := &DirAnalysis{
		AllFiles:    []string{"console.log", "syslog"},
		LogFiles:    []LogFileSnippet{{Path: "/logs/console.log", Content: "error here"}},
		FailedTasks: []api.FailedTask{{Task: "install", Host: "h1"}},
		LogContext:  []LogBlock{{Lines: []LogLine{{N: 5, Text: "fatal: x", Match: true}}}},
	}

	prompt := BuildDirAnalysisPrompt(input, da)
	for _, want := range []string{"deploy", "myproject", "console.log", "error here", "install"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("missing %q in prompt", want)
		}
	}
}
