package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Status response from /api/tenant/{t}/status.
type Status struct {
	Pipelines            []Pipeline     `json:"pipelines"`
	ZuulVersion          string         `json:"zuul_version,omitempty"`
	TriggerEventQueue    map[string]any `json:"trigger_event_queue,omitempty"`
	ManagementEventQueue map[string]any `json:"management_event_queue,omitempty"`
	ResultEventQueue     map[string]any `json:"result_event_queue,omitempty"`
	LastReconfigured     json.Number    `json:"last_reconfigured,omitempty"`
}

type Pipeline struct {
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ChangeQueues []ChangeQueue `json:"change_queues"`
}

type ChangeQueue struct {
	Name  string        `json:"name"`
	Heads [][]QueueItem `json:"heads"`
}

type QueueItem struct {
	ID             any         `json:"id"`
	Project        any         `json:"project"`
	Refs           []QueueRef  `json:"refs"`
	EnqueueTime    json.Number `json:"enqueue_time,omitempty"`
	RemainingTime  json.Number `json:"remaining_time,omitempty"`
	Jobs           []JobStatus `json:"jobs"`
	Active         bool        `json:"active"`
	Live           bool        `json:"live"`
	FailingReasons []string    `json:"failing_reasons,omitempty"`
}

// QueueRef holds the ref-level metadata for a queue item.
// The project name and change ID live here rather than at the top level.
type QueueRef struct {
	ID               string `json:"id"`
	Project          string `json:"project"`
	ProjectCanonical string `json:"project_canonical"`
	Owner            string `json:"owner"`
	Ref              string `json:"ref"`
	URL              string `json:"url"`
}

// ProjectName returns the project name, preferring refs[0].project
// over the top-level project field.
func (q *QueueItem) ProjectName() string {
	if len(q.Refs) > 0 && q.Refs[0].Project != "" {
		return q.Refs[0].Project
	}
	if q.Project == nil {
		return ""
	}
	switch v := q.Project.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["canonical_name"].(string); ok && name != "" {
			return name
		}
		if name, ok := v["name"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

// ChangeID returns a short human-readable change identifier.
// For merge requests it extracts the MR number from refs[0].id ("215,hash" → "215").
// Falls back to the top-level id.
func (q *QueueItem) ChangeID() string {
	if len(q.Refs) > 0 && q.Refs[0].ID != "" {
		rid := q.Refs[0].ID
		if idx := strings.Index(rid, ","); idx > 0 {
			return rid[:idx]
		}
		return rid
	}
	return fmt.Sprintf("%v", q.ID)
}

// Owner returns the change owner from refs[0].owner.
func (q *QueueItem) Owner() string {
	if len(q.Refs) > 0 {
		return q.Refs[0].Owner
	}
	return ""
}

// RefName returns the ref string (e.g. "refs/merge-requests/215/head").
func (q *QueueItem) RefName() string {
	if len(q.Refs) > 0 {
		return q.Refs[0].Ref
	}
	return ""
}

// ChangeURL returns the URL to the change (merge request, PR, etc.)
// from refs[0].url, or empty string if unavailable.
func (q *QueueItem) ChangeURL() string {
	if len(q.Refs) > 0 {
		return q.Refs[0].URL
	}
	return ""
}

type JobStatus struct {
	Name          string      `json:"name"`
	URL           string      `json:"url,omitempty"`
	UUID          string      `json:"uuid,omitempty"`
	ReportURL     string      `json:"report_url,omitempty"`
	Result        *string     `json:"result"`
	Voting        bool        `json:"voting"`
	StartTime     json.Number `json:"start_time,omitempty"`
	ElapsedTime   json.Number `json:"elapsed_time,omitempty"`
	RemainingTime json.Number `json:"remaining_time,omitempty"`
	EstimatedTime json.Number `json:"estimated_time,omitempty"`
}

func (c *Client) GetStatus() (*Status, error) {
	var status Status
	err := c.getJSON(c.tenantPath("status"), nil, &status)
	return &status, err
}

func (c *Client) GetStatusChange(change string) (*Status, error) {
	var status Status
	err := c.getJSON(c.tenantPath("status/change/"+change), nil, &status)
	return &status, err
}

type PipelineInfo struct {
	Name string `json:"name"`
}

func (c *Client) GetPipelines() ([]PipelineInfo, error) {
	var pipelines []PipelineInfo
	err := c.getJSON(c.tenantPath("pipelines"), nil, &pipelines)
	return pipelines, err
}

func (c *Client) GetTenantStatus() (map[string]any, error) {
	var result map[string]any
	err := c.getJSON(c.tenantPath("tenant-status"), nil, &result)
	return result, err
}

func (c *Client) GetInfo() (map[string]any, error) {
	var result map[string]any
	err := c.getJSON(c.tenantPath("info"), nil, &result)
	return result, err
}

func (c *Client) GetSystemEvents(limit, skip int) ([]SystemEvent, error) {
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	var events []SystemEvent
	err := c.getJSON(c.tenantPath("system-events"), params, &events)
	return events, err
}

type SystemEvent struct {
	EventID     string `json:"event_id"`
	EventType   string `json:"event_type"`
	EventTime   string `json:"event_time"`
	Description string `json:"description"`
}
