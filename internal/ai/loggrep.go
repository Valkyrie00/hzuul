package ai

import (
	"regexp"
	"strings"
)

var fatalPattern = regexp.MustCompile(`(?i)fatal:|FAILED!`)

type LogLine struct {
	N     int
	Text  string
	Match bool
}

type LogBlock struct {
	Lines []LogLine
}

func GrepLogContext(text string, contextLines int) []LogBlock {
	if contextLines <= 0 {
		contextLines = 3
	}
	allLines := strings.Split(text, "\n")
	total := len(allLines)

	type matched struct {
		idx  int
		text string
	}
	var matches []matched

	for i, line := range allLines {
		if fatalPattern.MatchString(line) {
			if isIgnoredFatal(allLines, i, total) {
				continue
			}
			matches = append(matches, matched{i + 1, line})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	if len(matches) > 15 {
		matches = matches[len(matches)-15:]
	}

	matchSet := make(map[int]struct{}, len(matches))
	for _, m := range matches {
		matchSet[m.idx-1] = struct{}{}
	}

	type rng struct{ start, end int }
	var ranges []rng
	for _, m := range matches {
		start := m.idx - 1 - contextLines
		if start < 0 {
			start = 0
		}
		end := m.idx + contextLines
		if end > total {
			end = total
		}
		if len(ranges) > 0 && start <= ranges[len(ranges)-1].end {
			ranges[len(ranges)-1].end = max(ranges[len(ranges)-1].end, end)
		} else {
			ranges = append(ranges, rng{start, end})
		}
	}

	if len(ranges) > 7 {
		ranges = ranges[len(ranges)-7:]
	}

	blocks := make([]LogBlock, 0, len(ranges))
	for _, r := range ranges {
		lines := make([]LogLine, 0, r.end-r.start)
		for i := r.start; i < r.end; i++ {
			text := allLines[i]
			if len(text) > 300 {
				text = text[:300]
			}
			_, isMatch := matchSet[i]
			lines = append(lines, LogLine{
				N:     i + 1,
				Text:  text,
				Match: isMatch,
			})
		}
		blocks = append(blocks, LogBlock{Lines: lines})
	}
	return blocks
}

// isIgnoredFatal returns true when a fatal: line is followed (within the
// next two non-empty lines) by Ansible's "...ignoring" marker.
func isIgnoredFatal(lines []string, idx, total int) bool {
	for j := idx + 1; j < total && j <= idx+2; j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			continue
		}
		return strings.Contains(trimmed, "...ignoring")
	}
	return false
}

func StripANSI(text string) string {
	return ansiRe.ReplaceAllString(text, "")
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
