package api

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type Node struct {
	ID             string `json:"id"`
	Label          string `json:"label,omitempty"`
	Connection     string `json:"connection,omitempty"`
	Type           any    `json:"type,omitempty"`
	Provider       string `json:"provider,omitempty"`
	State          string `json:"state,omitempty"`
	StateTime      any    `json:"state_time,omitempty"`
	Comment        string `json:"comment,omitempty"`
	Cloud          string `json:"cloud,omitempty"`
	Region         string `json:"region,omitempty"`
	HoldExpiration any    `json:"hold_expiration,omitempty"`
	RequestTime    any    `json:"request_time,omitempty"`
}

func (n Node) TypeString() string {
	switch v := n.Type.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		if n.Type == nil {
			return ""
		}
		return fmt.Sprintf("%v", n.Type)
	}
}

func (n Node) DisplayLabel() string {
	if n.Label != "" {
		return n.Label
	}
	return n.TypeString()
}

var stateTimeFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
}

func parseTimeField(v any) (time.Time, bool) {
	switch t := v.(type) {
	case string:
		for _, layout := range stateTimeFormats {
			if parsed, err := time.Parse(layout, t); err == nil {
				return parsed, true
			}
		}
	case float64:
		if t > 0 {
			return time.Unix(int64(t), 0), true
		}
	}
	return time.Time{}, false
}

func (n Node) AgeString() string {
	t, ok := parseTimeField(n.StateTime)
	if !ok {
		t, ok = parseTimeField(n.RequestTime)
	}
	if !ok {
		return "-"
	}
	secs := time.Since(t).Seconds()
	if secs <= 0 {
		return "-"
	}
	total := int(math.Round(secs))
	d := total / 86400
	h := (total % 86400) / 3600
	m := (total % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh", d, h)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func (c *Client) GetNodes() ([]Node, error) {
	var nodes []Node
	err := c.getJSON(c.tenantPath("nodes"), nil, &nodes)
	return nodes, err
}
