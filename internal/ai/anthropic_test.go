package ai

import (
	"strings"
	"testing"
)

func TestFormatAPIError_KnownCodes(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{401, "authentication failed"},
		{403, "access denied"},
		{404, "not found"},
		{429, "rate limit"},
		{500, "temporarily unavailable"},
		{502, "temporarily unavailable"},
		{503, "temporarily unavailable"},
	}
	for _, tt := range tests {
		got := formatAPIError(tt.status, "", "test-model")
		if !strings.Contains(got, tt.want) {
			t.Errorf("status %d: got %q, want substring %q", tt.status, got, tt.want)
		}
	}
}

func TestFormatAPIError_JSONErrorMessage(t *testing.T) {
	body := `{"error":{"message":"quota exceeded"}}`
	got := formatAPIError(418, body, "m")
	if got != "quota exceeded" {
		t.Errorf("got %q", got)
	}
}

func TestFormatAPIError_JSONTopLevelMessage(t *testing.T) {
	body := `{"message":"bad request"}`
	got := formatAPIError(400, body, "m")
	if got != "bad request" {
		t.Errorf("got %q", got)
	}
}

func TestFormatAPIError_TruncatesLongBody(t *testing.T) {
	body := strings.Repeat("x", 300)
	got := formatAPIError(418, body, "m")
	if len(got) > 210 {
		t.Errorf("not truncated: len=%d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("expected ... suffix")
	}
}

func TestFormatAPIError_ShortBody(t *testing.T) {
	got := formatAPIError(418, "oops", "m")
	if got != "oops" {
		t.Errorf("got %q", got)
	}
}
