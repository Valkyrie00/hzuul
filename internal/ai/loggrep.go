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

	matchSet := make(map[int]struct{})
	type matched struct {
		idx  int
		text string
	}
	var matches []matched

	for i, line := range allLines {
		if fatalPattern.MatchString(line) {
			matchSet[i] = struct{}{}
			matches = append(matches, matched{i + 1, line})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Limit to first 15 matches
	if len(matches) > 15 {
		matches = matches[:15]
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

	// Limit to 7 blocks
	if len(ranges) > 7 {
		ranges = ranges[:7]
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

func StripANSI(text string) string {
	return ansiRe.ReplaceAllString(text, "")
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
