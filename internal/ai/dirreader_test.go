package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Valkyrie00/hzuul/internal/api"
)

func TestSelectRemoteTargets_FiltersByExtension(t *testing.T) {
	entries := []api.FileEntry{
		{Path: "/logs/console.log", Size: 1000},
		{Path: "/logs/image.png", Size: 500},
		{Path: "/logs/data.json", Size: 800},
	}
	got := selectRemoteTargets(entries)
	for _, f := range got {
		if strings.HasSuffix(f.Path, ".png") {
			t.Error("should not include .png files")
		}
	}
}

func TestSelectRemoteTargets_SkipsJobOutput(t *testing.T) {
	entries := []api.FileEntry{
		{Path: "/logs/job-output.json", Size: 1000},
		{Path: "/logs/console.log", Size: 500},
	}
	got := selectRemoteTargets(entries)
	for _, f := range got {
		if strings.HasSuffix(f.Path, "job-output.json") {
			t.Error("should skip job-output.json")
		}
	}
}

func TestSelectRemoteTargets_SkipsOversized(t *testing.T) {
	entries := []api.FileEntry{
		{Path: "/logs/huge.log", Size: 30 * 1024 * 1024},
		{Path: "/logs/small.log", Size: 100},
	}
	got := selectRemoteTargets(entries)
	if len(got) != 1 || got[0].Path != "/logs/small.log" {
		t.Errorf("got %v", got)
	}
}

func TestSelectRemoteTargets_SkipsZeroSize(t *testing.T) {
	entries := []api.FileEntry{
		{Path: "/logs/empty.log", Size: 0},
		{Path: "/logs/ok.log", Size: 100},
	}
	got := selectRemoteTargets(entries)
	if len(got) != 1 {
		t.Errorf("expected 1, got %d", len(got))
	}
}

func TestSelectRemoteTargets_PrioritizesKnownFiles(t *testing.T) {
	entries := []api.FileEntry{
		{Path: "/logs/random.log", Size: 5000},
		{Path: "/logs/job-output.txt", Size: 1000},
	}
	got := selectRemoteTargets(entries)
	if len(got) < 1 {
		t.Fatal("expected at least 1 result")
	}
}

func TestPrioritizeFiles(t *testing.T) {
	files := []string{"/tmp/random.log", "/tmp/console.log", "/tmp/other.txt"}
	got := prioritizeFiles(files)
	if got[0] != "/tmp/console.log" {
		t.Errorf("expected console.log first, got %q", got[0])
	}
}

func TestPrioritizeFiles_Empty(t *testing.T) {
	got := prioritizeFiles(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func TestExtractRelevantSnippet_Short(t *testing.T) {
	content := "line1\nline2\nline3"
	got := extractRelevantSnippet(content)
	if got != content {
		t.Errorf("short content should be returned as-is")
	}
}

func TestExtractRelevantSnippet_Long(t *testing.T) {
	lines := make([]string, maxSnippetLines+100)
	for i := range lines {
		lines[i] = "line"
	}
	got := extractRelevantSnippet(strings.Join(lines, "\n"))
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != maxSnippetLines {
		t.Errorf("expected %d lines, got %d", maxSnippetLines, len(gotLines))
	}
}

func TestExtractErrorCenteredSnippet_Short(t *testing.T) {
	content := "line1\nfatal: boom\nline3"
	got := extractErrorCenteredSnippet(content)
	if got != content {
		t.Errorf("short content should be returned as-is")
	}
}

func TestExtractErrorCenteredSnippet_CentersOnError(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	lines[400] = "fatal: the real error"
	content := strings.Join(lines, "\n")
	got := extractErrorCenteredSnippet(content)
	if !strings.Contains(got, "fatal: the real error") {
		t.Error("snippet should contain the error line")
	}
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != maxErrorSnippetLines {
		t.Errorf("expected %d lines, got %d", maxErrorSnippetLines, len(gotLines))
	}
}

func TestExtractErrorCenteredSnippet_SkipsIgnored(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	lines[400] = "fatal: ignored error"
	lines[401] = "...ignoring"
	lines[800] = "fatal: real error"
	content := strings.Join(lines, "\n")
	got := extractErrorCenteredSnippet(content)
	if !strings.Contains(got, "fatal: real error") {
		t.Error("snippet should center on the non-ignored error")
	}
}

func TestExtractErrorCenteredSnippet_FallsBackToTail(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	content := strings.Join(lines, "\n")
	got := extractErrorCenteredSnippet(content)
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != maxSnippetLines {
		t.Errorf("expected %d tail lines, got %d", maxSnippetLines, len(gotLines))
	}
	if !strings.Contains(got, "line 999") {
		t.Error("tail fallback should include the last line")
	}
}

func TestMergeStats(t *testing.T) {
	output := []api.PlaybookOutput{
		{Stats: map[string]api.HostStats{"h1": {Ok: 1, Failures: 2}}},
		{Stats: map[string]api.HostStats{"h1": {Ok: 3, Failures: 1}}},
	}
	got := mergeStats(output)
	if got["h1"].Ok != 4 || got["h1"].Failures != 3 {
		t.Errorf("got %+v", got["h1"])
	}
}

func TestMergeStats_Empty(t *testing.T) {
	got := mergeStats(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

func setupTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "console.log"), []byte("line1\nfatal: boom\nline3"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "syslog"), []byte("ok"), 0o644)
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("binary"), 0o644)
	return dir
}

func TestListAllFiles(t *testing.T) {
	dir := setupTempDir(t)
	files := listAllFiles(dir)
	want := map[string]bool{"console.log": false, filepath.Join("sub", "syslog"): false, "image.png": false}
	for _, f := range files {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing file: %q (got %v)", name, files)
		}
	}
	if len(files) != 3 {
		t.Errorf("expected exactly 3 files, got %d: %v", len(files), files)
	}
}

func TestFindFile(t *testing.T) {
	dir := setupTempDir(t)
	got := findFile(dir, "syslog")
	if got == "" {
		t.Fatal("expected to find syslog")
	}
	if filepath.Base(got) != "syslog" {
		t.Errorf("found wrong file: %q", got)
	}
	if !strings.HasPrefix(got, dir) {
		t.Errorf("path should be under temp dir: %q", got)
	}
}

func TestFindFile_NotFound(t *testing.T) {
	dir := setupTempDir(t)
	got := findFile(dir, "nonexistent.txt")
	if got != "" {
		t.Errorf("expected empty for missing file, got %q", got)
	}
}

func TestCollectLogFiles_FiltersCorrectly(t *testing.T) {
	dir := setupTempDir(t)
	files := collectLogFiles(dir)

	basenames := make(map[string]bool)
	for _, f := range files {
		basenames[filepath.Base(f)] = true
	}
	if basenames["image.png"] {
		t.Error("collected .png file, should only include log-like extensions")
	}
	if !basenames["console.log"] {
		t.Error("missing console.log")
	}
	if !basenames["syslog"] {
		t.Error("missing syslog (extensionless files should be included)")
	}
}

func TestCollectLogFiles_SkipsJobOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "job-output.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "console.log"), []byte("x"), 0o644)
	files := collectLogFiles(dir)
	for _, f := range files {
		if filepath.Base(f) == "job-output.json" {
			t.Error("collected job-output.json, should be excluded")
		}
	}
	if len(files) != 1 {
		t.Errorf("expected exactly 1 file (console.log), got %d: %v", len(files), files)
	}
}

func TestCollectLogFiles_SkipsEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.log"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(dir, "notempty.log"), []byte("data"), 0o644)
	files := collectLogFiles(dir)
	for _, f := range files {
		if filepath.Base(f) == "empty.log" {
			t.Error("collected empty file, should skip size=0")
		}
	}
}

func TestReadLogsFromDir(t *testing.T) {
	dir := t.TempDir()
	output := []api.PlaybookOutput{
		{
			Playbook: "site.yml",
			Phase:    "run",
			Stats: map[string]api.HostStats{
				"host1": {Ok: 5, Failures: 1},
			},
			Plays: []api.PlayOutput{{
				Tasks: []api.TaskOutput{{
					Task:  map[string]any{"name": "compile"},
					Hosts: map[string]api.TaskHostResult{"host1": {Failed: true, Msg: "exit code 1"}},
				}},
			}},
		},
	}
	jobOutputJSON, _ := json.Marshal(output)
	os.WriteFile(filepath.Join(dir, "job-output.json"), jobOutputJSON, 0o644)
	os.WriteFile(filepath.Join(dir, "console.log"), []byte("line1\nfatal: error\nline3"), 0o644)

	da, err := ReadLogsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(da.JobOutput) != 1 || da.JobOutput[0].Playbook != "site.yml" {
		t.Errorf("JobOutput = %+v", da.JobOutput)
	}
	if len(da.FailedTasks) != 1 || da.FailedTasks[0].Task != "compile" || da.FailedTasks[0].Msg != "exit code 1" {
		t.Errorf("FailedTasks = %+v", da.FailedTasks)
	}
	foundConsole := false
	for _, lf := range da.LogFiles {
		if strings.Contains(lf.Path, "console.log") {
			foundConsole = true
			if !strings.Contains(lf.Content, "fatal: error") {
				t.Errorf("console.log content missing fatal line: %q", lf.Content)
			}
		}
	}
	if !foundConsole {
		t.Errorf("LogFiles missing console.log: %+v", da.LogFiles)
	}
	foundAll := false
	for _, f := range da.AllFiles {
		if strings.Contains(f, "console.log") {
			foundAll = true
		}
	}
	if !foundAll {
		t.Errorf("AllFiles missing console.log: %v", da.AllFiles)
	}
}

func TestReadLogsFromDir_NoJobOutput(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "console.log"), []byte("fatal: something went wrong"), 0o644)

	da, err := ReadLogsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if da.JobOutput != nil {
		t.Errorf("expected nil JobOutput, got %+v", da.JobOutput)
	}
	if len(da.FailedTasks) != 0 {
		t.Errorf("expected no FailedTasks without job-output.json, got %+v", da.FailedTasks)
	}
	if len(da.LogFiles) == 0 {
		t.Fatal("expected log file snippets even without job-output.json")
	}
	if !strings.Contains(da.LogFiles[0].Content, "fatal: something went wrong") {
		t.Errorf("log content = %q", da.LogFiles[0].Content)
	}
}

func TestReadLogsFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	da, err := ReadLogsFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(da.LogFiles) != 0 {
		t.Errorf("expected no LogFiles in empty dir, got %d", len(da.LogFiles))
	}
}
