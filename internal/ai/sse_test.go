package ai

import (
	"strings"
	"testing"
)

func TestParseSSE_TextDeltas(t *testing.T) {
	stream := `event: message_start
data: {"type":"message_start"}

data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}

data: [DONE]
`
	var chunks []string
	err := ParseSSE(strings.NewReader(stream), func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0] != "Hello" || chunks[1] != " world" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestParseSSE_SkipsNonTextEvents(t *testing.T) {
	stream := `data: {"type":"message_start"}
data: {"type":"content_block_start"}
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"ok"}}
data: {"type":"message_stop"}
data: [DONE]
`
	var chunks []string
	err := ParseSSE(strings.NewReader(stream), func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0] != "ok" {
		t.Errorf("chunks = %v, want [ok]", chunks)
	}
}

func TestParseSSE_EmptyStream(t *testing.T) {
	var chunks []string
	err := ParseSSE(strings.NewReader(""), func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected no chunks, got %d", len(chunks))
	}
}

func TestParseSSE_MalformedJSON(t *testing.T) {
	stream := "data: {not json}\ndata: [DONE]\n"
	err := ParseSSE(strings.NewReader(stream), func(string) {})
	if err != nil {
		t.Fatalf("malformed JSON should be skipped, got error: %v", err)
	}
}

func TestParseGeminiSSE(t *testing.T) {
	stream := `data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":" Gemini"}]}}]}

data: [DONE]
`
	var chunks []string
	err := parseGeminiSSE(strings.NewReader(stream), func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 || chunks[0] != "Hello" || chunks[1] != " Gemini" {
		t.Errorf("chunks = %v", chunks)
	}
}

func TestParseGeminiSSE_EmptyCandidates(t *testing.T) {
	stream := `data: {"candidates":[]}
data: [DONE]
`
	var chunks []string
	err := parseGeminiSSE(strings.NewReader(stream), func(s string) {
		chunks = append(chunks, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected no chunks, got %d", len(chunks))
	}
}
