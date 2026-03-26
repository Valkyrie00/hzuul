package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Build struct {
	ID          any        `json:"_id"`
	UUID        string     `json:"uuid"`
	JobName     string     `json:"job_name"`
	Pipeline    string     `json:"pipeline"`
	Result      string     `json:"result"`
	StartTime   string     `json:"start_time"`
	EndTime     string     `json:"end_time"`
	Duration    any        `json:"duration"`
	Voting      any        `json:"voting"`
	LogURL      string     `json:"log_url"`
	Nodeset     string     `json:"nodeset"`
	ErrorDetail string     `json:"error_detail,omitempty"`
	Final       any        `json:"final"`
	Held        any        `json:"held"`
	Ref         BuildRef   `json:"ref"`
	Artifacts   []Artifact `json:"artifacts,omitempty"`
}

type BuildRef struct {
	Project  string `json:"project"`
	Branch   string `json:"branch"`
	Change   any    `json:"change"`
	Patchset any    `json:"patchset"`
	Ref      string `json:"ref"`
	RefURL   string `json:"ref_url"`
	Newrev   string `json:"newrev"`
	Oldrev   string `json:"oldrev"`
}

type Artifact struct {
	Name     string         `json:"name"`
	URL      string         `json:"url"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type BuildFilter struct {
	Project       string
	Pipeline      string
	JobName       string
	Branch        string
	Change        string
	Patchset      string
	Result        string
	Limit         int
	Skip          int
	ExcludeResult string
}

func (f *BuildFilter) toParams() url.Values {
	p := url.Values{}
	if f == nil {
		return p
	}
	if f.Project != "" {
		p.Set("project", f.Project)
	}
	if f.Pipeline != "" {
		p.Set("pipeline", f.Pipeline)
	}
	if f.JobName != "" {
		p.Set("job_name", f.JobName)
	}
	if f.Branch != "" {
		p.Set("branch", f.Branch)
	}
	if f.Change != "" {
		p.Set("change", f.Change)
	}
	if f.Patchset != "" {
		p.Set("patchset", f.Patchset)
	}
	if f.Result != "" {
		p.Set("result", f.Result)
	}
	if f.Limit > 0 {
		p.Set("limit", fmt.Sprintf("%d", f.Limit))
	}
	if f.Skip > 0 {
		p.Set("skip", fmt.Sprintf("%d", f.Skip))
	}
	if f.ExcludeResult != "" {
		p.Set("exclude_result", f.ExcludeResult)
	}
	return p
}

func (c *Client) GetBuilds(filter *BuildFilter) ([]Build, error) {
	var builds []Build
	err := c.getJSON(c.tenantPath("builds"), filter.toParams(), &builds)
	return builds, err
}

func (c *Client) GetBuild(uuid string) (*Build, error) {
	var build Build
	err := c.getJSON(c.tenantPath("build/"+uuid), nil, &build)
	return &build, err
}

// PlaybookOutput represents one playbook entry from job-output.json.
type PlaybookOutput struct {
	Playbook string                    `json:"playbook"`
	Phase    string                    `json:"phase"`
	Stats    map[string]HostStats      `json:"stats"`
	Plays    []PlayOutput              `json:"plays"`
}

type PlayOutput struct {
	Play  map[string]any `json:"play"`
	Tasks []TaskOutput   `json:"tasks"`
}

type TaskOutput struct {
	Task  map[string]any            `json:"task"`
	Hosts map[string]TaskHostResult `json:"hosts"`
}

type TaskHostResult struct {
	Failed       bool   `json:"failed"`
	Action       string `json:"action"`
	Cmd          any    `json:"cmd"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	Msg          any    `json:"msg"`
	StdoutLines  []any  `json:"stdout_lines"`
	StderrLines  []any  `json:"stderr_lines"`
	IgnoreErrors bool   `json:"_ansible_ignore_errors"`
}

func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, "\n")
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (r TaskHostResult) MsgString() string {
	return anyToString(r.Msg)
}

func (r TaskHostResult) CmdString() string {
	switch v := r.Cmd.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, p := range v {
			parts = append(parts, fmt.Sprintf("%v", p))
		}
		return strings.Join(parts, " ")
	default:
		if v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
}

type HostStats struct {
	Ok          int `json:"ok"`
	Changed     int `json:"changed"`
	Failures    int `json:"failures"`
	Skipped     int `json:"skipped"`
	Unreachable int `json:"unreachable"`
	Rescued     int `json:"rescued"`
	Ignored     int `json:"ignored"`
}

type FailedTask struct {
	Playbook string
	Task     string
	Host     string
	Action   string
	Cmd      string
	Msg      string
	Stdout   string
	Stderr   string
}

func (c *Client) GetJobOutput(logURL string) ([]PlaybookOutput, error) {
	u := strings.TrimRight(logURL, "/") + "/job-output.json"

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching job-output.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("job-output.json: %s — %s", resp.Status, string(body))
	}

	var output []PlaybookOutput
	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return nil, fmt.Errorf("decoding job-output.json: %w", err)
	}
	return output, nil
}

// AggregateStats merges per-host stats across all playbooks.
func AggregateStats(output []PlaybookOutput) map[string]HostStats {
	merged := make(map[string]HostStats)
	for _, pb := range output {
		for host, s := range pb.Stats {
			m := merged[host]
			m.Ok += s.Ok
			m.Changed += s.Changed
			m.Failures += s.Failures
			m.Skipped += s.Skipped
			m.Unreachable += s.Unreachable
			m.Rescued += s.Rescued
			m.Ignored += s.Ignored
			merged[host] = m
		}
	}
	return merged
}

// ExtractFailedTasks collects tasks that actually caused failures,
// filtering out tasks where ignore_errors was set or the host has no
// real failures in the aggregated stats (e.g. rescued tasks).
func ExtractFailedTasks(output []PlaybookOutput, stats map[string]HostStats) []FailedTask {
	var failed []FailedTask
	for _, pb := range output {
		for _, play := range pb.Plays {
			for _, task := range play.Tasks {
				taskName, _ := task.Task["name"].(string)
				for host, result := range task.Hosts {
					if !result.Failed || result.IgnoreErrors {
						continue
					}
					if hs, ok := stats[host]; ok && hs.Failures == 0 {
						continue
					}
					msg := result.MsgString()
					if msg == "" && result.Stderr != "" {
						msg = result.Stderr
					}
					failed = append(failed, FailedTask{
						Playbook: pb.Playbook,
						Task:     taskName,
						Host:     host,
						Action:   result.Action,
						Cmd:      result.CmdString(),
						Msg:      msg,
						Stdout:   result.Stdout,
						Stderr:   result.Stderr,
					})
				}
			}
		}
	}
	return failed
}

type Buildset struct {
	ID                 any          `json:"_id"`
	UUID               string       `json:"uuid"`
	Result             string       `json:"result"`
	Message            string       `json:"message,omitempty"`
	Pipeline           string       `json:"pipeline"`
	EventID            string       `json:"event_id,omitempty"`
	EventTimestamp     string       `json:"event_timestamp,omitempty"`
	FirstBuildStart    string       `json:"first_build_start_time,omitempty"`
	LastBuildEnd       string       `json:"last_build_end_time,omitempty"`
	Refs               []BuildRef   `json:"refs,omitempty"`
	Builds             []Build      `json:"builds,omitempty"`
}

func (c *Client) GetBuildsets(filter *BuildFilter) ([]Buildset, error) {
	var buildsets []Buildset
	err := c.getJSON(c.tenantPath("buildsets"), filter.toParams(), &buildsets)
	return buildsets, err
}

func (c *Client) GetBuildset(uuid string) (*Buildset, error) {
	var buildset Buildset
	err := c.getJSON(c.tenantPath("buildset/"+uuid), nil, &buildset)
	return &buildset, err
}
