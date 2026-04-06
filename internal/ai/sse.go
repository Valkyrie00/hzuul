package ai

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type sseEvent struct {
	Type  string          `json:"type"`
	Delta json.RawMessage `json:"delta,omitempty"`
}

type sseTextDelta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseSSE reads an Anthropic-compatible SSE stream and calls onChunk
// for each text delta. Reusable across any provider that speaks this format.
func ParseSSE(r io.Reader, onChunk func(string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event sseEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		if event.Type == "content_block_delta" && event.Delta != nil {
			var delta sseTextDelta
			if err := json.Unmarshal(event.Delta, &delta); err == nil && delta.Text != "" {
				onChunk(delta.Text)
			}
		}
	}
	return scanner.Err()
}
