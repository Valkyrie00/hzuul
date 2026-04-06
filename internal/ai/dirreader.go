package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Valkyrie00/hzuul/internal/api"
)

const maxLogFileSize = 512 * 1024 // 512 KB per file
const maxTotalLogContext = 128 * 1024 // 128 KB total for the prompt

type DirAnalysis struct {
	JobOutput   []api.PlaybookOutput
	FailedTasks []api.FailedTask
	LogContext  []LogBlock
	LogFiles    []LogFileSnippet
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

	logFiles := collectLogFiles(destDir)
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
		if ext == ".txt" || ext == ".log" || ext == ".json" || ext == "" {
			if info.Size() > 0 && info.Size() < 10*1024*1024 {
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

func extractRelevantSnippet(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= 60 {
		return content
	}
	return strings.Join(lines[len(lines)-60:], "\n")
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
