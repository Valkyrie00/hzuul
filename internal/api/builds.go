package api

import (
	"fmt"
	"net/url"
)

type Build struct {
	ID          string     `json:"_id"`
	UUID        string     `json:"uuid"`
	JobName     string     `json:"job_name"`
	Result      string     `json:"result"`
	StartTime   string     `json:"start_time"`
	EndTime     string     `json:"end_time"`
	Duration    string     `json:"duration"`
	Voting      string     `json:"voting"`
	LogURL      string     `json:"log_url"`
	Nodeset     string     `json:"nodeset"`
	ErrorDetail string     `json:"error_detail,omitempty"`
	Final       string     `json:"final"`
	Held        string     `json:"held"`
	Ref         BuildRef   `json:"ref"`
	Artifacts   []Artifact `json:"artifacts,omitempty"`
}

type BuildRef struct {
	Project  string `json:"project"`
	Branch   string `json:"branch"`
	Change   string `json:"change"`
	Patchset string `json:"patchset"`
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

type Buildset struct {
	ID                 string       `json:"_id"`
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
