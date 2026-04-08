package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/api"
)

const maxLogFileSize = 1024 * 1024   // 1 MB per file (tail)
const maxTotalLogContext = 768 * 1024 // 768 KB total for the prompt
const maxSnippetLines = 300

// priorityFiles are read first and given more generous space.
var priorityFiles = []string{
	"console.log",
	"job-output.json",
	"tempest.log",
	"devstack.log",
	"devstack-gate.log",
	"syslog",
}

type DirAnalysis struct {
	JobOutput   []api.PlaybookOutput
	FailedTasks []api.FailedTask
	LogContext  []LogBlock
	LogFiles    []LogFileSnippet
	AllFiles    []string
}

type LogFileSnippet struct {
	Path    string
	Content string
}

func ReadLogsFromDir(destDir string) (*DirAnalysis, error) {
	result := &DirAnalysis{}

	joPath := findFile(destDir, "job-output.json")
	if joPath != "" {
		data, err := os.ReadFile(joPath)
		if err == nil {
			var output []api.PlaybookOutput
			if json.Unmarshal(data, &output) == nil {
				result.JobOutput = output
				allStats := mergeStats(output)
				result.FailedTasks = api.ExtractFailedTasks(output, allStats)
			}
		}
	}

	result.AllFiles = listAllFiles(destDir)

	logFiles := collectLogFiles(destDir)
	logFiles = prioritizeFiles(logFiles)

	var totalCtx int
	for _, lf := range logFiles {
		if totalCtx >= maxTotalLogContext {
			break
		}
		data, err := os.ReadFile(lf)
		if err != nil {
			continue
		}
		content := StripANSI(string(data))
		if len(content) > maxLogFileSize {
			content = content[len(content)-maxLogFileSize:]
		}

		blocks := GrepLogContext(content, 5)
		if len(blocks) > 0 {
			result.LogContext = append(result.LogContext, blocks...)
		}

		rel, _ := filepath.Rel(destDir, lf)
		if rel == "" {
			rel = filepath.Base(lf)
		}

		snippet := extractRelevantSnippet(content)
		if snippet != "" && totalCtx+len(snippet) <= maxTotalLogContext {
			result.LogFiles = append(result.LogFiles, LogFileSnippet{
				Path:    rel,
				Content: snippet,
			})
			totalCtx += len(snippet)
		}
	}

	return result, nil
}

func listAllFiles(root string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel != "" {
			files = append(files, rel)
		}
		return nil
	})
	return files
}

func findFile(root, name string) string {
	var found string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return filepath.SkipDir
		}
		if !info.IsDir() && info.Name() == name {
			found = path
			return filepath.SkipDir
		}
		return nil
	})
	return found
}

func collectLogFiles(root string) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == "job-output.json" {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".txt", ".log", ".json", ".conf", ".yaml", ".yml", ".html", ".xml", ".csv", "":
			if info.Size() > 0 && info.Size() < 20*1024*1024 {
				files = append(files, path)
			}
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		fi, _ := os.Stat(files[i])
		fj, _ := os.Stat(files[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return files
}

// prioritizeFiles moves known important log files to the front.
func prioritizeFiles(files []string) []string {
	prio := make([]string, 0, len(files))
	rest := make([]string, 0, len(files))
	prioSet := make(map[string]bool, len(priorityFiles))
	for _, p := range priorityFiles {
		prioSet[p] = true
	}
	for _, f := range files {
		if prioSet[filepath.Base(f)] {
			prio = append(prio, f)
		} else {
			rest = append(rest, f)
		}
	}
	return append(prio, rest...)
}

func extractRelevantSnippet(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxSnippetLines {
		return content
	}
	return strings.Join(lines[len(lines)-maxSnippetLines:], "\n")
}

func mergeStats(output []api.PlaybookOutput) map[string]api.HostStats {
	all := make(map[string]api.HostStats)
	for _, pb := range output {
		for host, st := range pb.Stats {
			cur := all[host]
			cur.Ok += st.Ok
			cur.Changed += st.Changed
			cur.Failures += st.Failures
			cur.Skipped += st.Skipped
			cur.Unreachable += st.Unreachable
			cur.Rescued += st.Rescued
			cur.Ignored += st.Ignored
			all[host] = cur
		}
	}
	return all
}
