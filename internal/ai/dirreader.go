package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Valkyrie00/hzuul/internal/api"
)

const maxLogFileSize = 1024 * 1024    // 1 MB per file for remote fetch
const maxTotalLogContext = 768 * 1024 // 768 KB total for the prompt
const maxSnippetLines = 300
const maxErrorSnippetLines = 500

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

		blocks := GrepLogContext(content, 5)
		if len(blocks) > 0 {
			result.LogContext = append(result.LogContext, blocks...)
		}

		rel, _ := filepath.Rel(destDir, lf)
		if rel == "" {
			rel = filepath.Base(lf)
		}

		snippet := extractErrorCenteredSnippet(content)
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

const maxRemoteFiles = 8

// ReadLogsFromRemote fetches key log files from a Zuul build's log server
// directly into memory (no disk writes) and returns the same DirAnalysis
// structure used by the local reader.
func ReadLogsFromRemote(client *api.Client, build *api.Build, jobOutput []api.PlaybookOutput) (*DirAnalysis, error) {
	result := &DirAnalysis{}

	if jobOutput != nil {
		result.JobOutput = jobOutput
		allStats := mergeStats(jobOutput)
		result.FailedTasks = api.ExtractFailedTasks(jobOutput, allStats)
	}

	manifestURL := api.GetManifestURL(build)
	if manifestURL == "" {
		return result, nil
	}

	manifest, err := client.FetchManifest(manifestURL)
	if err != nil {
		return result, nil
	}

	allEntries := api.CollectFiles(manifest.Tree)
	for _, fe := range allEntries {
		result.AllFiles = append(result.AllFiles, fe.Path)
	}

	targets := selectRemoteTargets(allEntries)
	baseLogURL := strings.TrimRight(build.LogURL, "/")

	type fetchResult struct {
		path    string
		content string
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var fetched []fetchResult

	for _, t := range targets {
		wg.Add(1)
		go func(fe api.FileEntry) {
			defer wg.Done()
			content, err := client.FetchFileContent(baseLogURL+fe.Path, maxLogFileSize)
			if err != nil || content == "" {
				return
			}
			mu.Lock()
			fetched = append(fetched, fetchResult{path: fe.Path, content: content})
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	var totalCtx int
	for _, fr := range fetched {
		if totalCtx >= maxTotalLogContext {
			break
		}
		content := StripANSI(fr.content)

		blocks := GrepLogContext(content, 5)
		if len(blocks) > 0 {
			result.LogContext = append(result.LogContext, blocks...)
		}

		snippet := extractErrorCenteredSnippet(content)
		if snippet != "" && totalCtx+len(snippet) <= maxTotalLogContext {
			result.LogFiles = append(result.LogFiles, LogFileSnippet{
				Path:    fr.path,
				Content: snippet,
			})
			totalCtx += len(snippet)
		}
	}

	return result, nil
}

// selectRemoteTargets picks the most important files from a manifest for remote fetch.
func selectRemoteTargets(entries []api.FileEntry) []api.FileEntry {
	prioSet := make(map[string]bool, len(priorityFiles))
	for _, p := range priorityFiles {
		prioSet[p] = true
	}

	var prio, rest []api.FileEntry
	for _, fe := range entries {
		name := fe.Path[strings.LastIndex(fe.Path, "/")+1:]
		if name == "job-output.json" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".txt", ".log", ".json", ".conf", ".yaml", ".yml", "":
		default:
			continue
		}
		if fe.Size <= 0 || fe.Size > 20*1024*1024 {
			continue
		}
		if prioSet[name] {
			prio = append(prio, fe)
		} else {
			rest = append(rest, fe)
		}
	}

	// Sort rest by size descending (larger files often have more useful info)
	sort.Slice(rest, func(i, j int) bool {
		return rest[i].Size > rest[j].Size
	})

	result := append(prio, rest...)
	if len(result) > maxRemoteFiles {
		result = result[:maxRemoteFiles]
	}
	return result
}

func listAllFiles(root string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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

// extractErrorCenteredSnippet finds the last real error (fatal/FAILED not
// followed by ...ignoring) and returns a window of maxErrorSnippetLines
// centered around it, biased toward showing context before the error.
// Falls back to a tail snippet when no error pattern is found.
func extractErrorCenteredSnippet(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxErrorSnippetLines {
		return content
	}

	total := len(lines)
	lastErrorIdx := -1
	for i := total - 1; i >= 0; i-- {
		if fatalPattern.MatchString(lines[i]) && !isIgnoredFatal(lines, i, total) {
			lastErrorIdx = i
			break
		}
	}

	if lastErrorIdx < 0 {
		return strings.Join(lines[total-maxSnippetLines:], "\n")
	}

	before := maxErrorSnippetLines * 2 / 3
	after := maxErrorSnippetLines - before

	start := lastErrorIdx - before
	end := lastErrorIdx + after
	if start < 0 {
		end = min(total, end-start)
		start = 0
	}
	if end > total {
		start = max(0, start-(end-total))
		end = total
	}

	return strings.Join(lines[start:end], "\n")
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
