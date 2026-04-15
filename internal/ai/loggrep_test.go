package ai

import (
	"strings"
	"testing"
)

func TestGrepLogContext_BasicMatch(t *testing.T) {
	lines := []string{
		"line 1",
		"line 2",
		"fatal: something broke",
		"line 4",
		"line 5",
	}
	blocks := GrepLogContext(strings.Join(lines, "\n"), 1)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	found := false
	for _, l := range blocks[0].Lines {
		if l.Match && strings.Contains(l.Text, "fatal:") {
			found = true
		}
	}
	if !found {
		t.Error("expected a matching line with fatal:")
	}
}

func TestGrepLogContext_NoMatch(t *testing.T) {
	blocks := GrepLogContext("all good\nnothing wrong\n", 3)
	if blocks != nil {
		t.Errorf("expected nil, got %d blocks", len(blocks))
	}
}

func TestGrepLogContext_FAILEDPattern(t *testing.T) {
	blocks := GrepLogContext("ok\nFAILED! => {\"msg\": \"oops\"}\nok", 1)
	if len(blocks) == 0 {
		t.Fatal("expected at least 1 block for FAILED! pattern")
	}
}

func TestGrepLogContext_TruncatesLongLines(t *testing.T) {
	long := "fatal: " + strings.Repeat("x", 500)
	blocks := GrepLogContext(long, 1)
	if len(blocks) == 0 {
		t.Fatal("expected at least 1 block")
	}
	for _, l := range blocks[0].Lines {
		if len(l.Text) > 300 {
			t.Errorf("line not truncated: len=%d", len(l.Text))
		}
	}
}

func TestGrepLogContext_MergesOverlappingRanges(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "ok")
	}
	lines[3] = "fatal: first"
	lines[5] = "fatal: second"

	blocks := GrepLogContext(strings.Join(lines, "\n"), 2)
	if len(blocks) != 1 {
		t.Errorf("expected 1 merged block, got %d", len(blocks))
	}
}

func TestGrepLogContext_DefaultContext(t *testing.T) {
	blocks := GrepLogContext("a\nb\nfatal: x\nc\nd", 0)
	if len(blocks) == 0 {
		t.Fatal("expected blocks with default context")
	}
}

func TestGrepLogContext_SkipsIgnoredFatals(t *testing.T) {
	lines := []string{
		"line 1",
		"fatal: expected failure",
		"...ignoring",
		"line 4",
		"fatal: real error",
		"line 6",
	}
	blocks := GrepLogContext(strings.Join(lines, "\n"), 1)
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 (only the non-ignored fatal)", len(blocks))
	}
	found := false
	for _, l := range blocks[0].Lines {
		if l.Match && strings.Contains(l.Text, "real error") {
			found = true
		}
		if l.Match && strings.Contains(l.Text, "expected failure") {
			t.Error("should not match ignored fatal")
		}
	}
	if !found {
		t.Error("expected match for 'real error'")
	}
}

func TestGrepLogContext_KeepsLastMatches(t *testing.T) {
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "ok")
	}
	for i := 0; i < 20; i++ {
		idx := i * 5
		lines[idx] = "fatal: error " + strings.Repeat("x", 1)
	}
	blocks := GrepLogContext(strings.Join(lines, "\n"), 1)
	if len(blocks) == 0 {
		t.Fatal("expected blocks")
	}
	lastBlock := blocks[len(blocks)-1]
	lastMatchLine := 0
	for _, l := range lastBlock.Lines {
		if l.Match {
			lastMatchLine = l.N
		}
	}
	if lastMatchLine < 90 {
		t.Errorf("expected last block near end of file, got line %d", lastMatchLine)
	}
}

func TestIsIgnoredFatal(t *testing.T) {
	lines := []string{
		"fatal: some error",
		"...ignoring",
		"ok",
		"fatal: real error",
		"next line",
	}
	if !isIgnoredFatal(lines, 0, len(lines)) {
		t.Error("line 0 should be ignored (next line has ...ignoring)")
	}
	if isIgnoredFatal(lines, 3, len(lines)) {
		t.Error("line 3 should NOT be ignored")
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"no ansi", "no ansi"},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := StripANSI(tt.input); got != tt.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
