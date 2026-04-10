package api

import (
	"testing"
)

func TestBuildFilter_ToParams(t *testing.T) {
	f := &BuildFilter{
		Project:       "openstack/nova",
		Pipeline:      "check",
		JobName:       "tox-py312",
		Branch:        "master",
		Change:        "12345",
		Patchset:      "3",
		Result:        "FAILURE",
		Limit:         50,
		Skip:          10,
		ExcludeResult: "SUCCESS",
	}
	p := f.toParams()
	checks := map[string]string{
		"project":        "openstack/nova",
		"pipeline":       "check",
		"job_name":       "tox-py312",
		"branch":         "master",
		"change":         "12345",
		"patchset":       "3",
		"result":         "FAILURE",
		"limit":          "50",
		"skip":           "10",
		"exclude_result": "SUCCESS",
	}
	for k, want := range checks {
		if got := p.Get(k); got != want {
			t.Errorf("param %q = %q, want %q", k, got, want)
		}
	}
}

func TestBuildFilter_ToParams_Empty(t *testing.T) {
	f := &BuildFilter{}
	p := f.toParams()
	if len(p) != 0 {
		t.Errorf("expected empty params, got %v", p)
	}
}

func TestBuildFilter_ToParams_Nil(t *testing.T) {
	var f *BuildFilter
	p := f.toParams()
	if len(p) != 0 {
		t.Errorf("expected empty params for nil filter, got %v", p)
	}
}

func TestAggregateStats(t *testing.T) {
	output := []PlaybookOutput{
		{
			Stats: map[string]HostStats{
				"host1": {Ok: 5, Failures: 1, Changed: 2},
				"host2": {Ok: 3, Skipped: 1},
			},
		},
		{
			Stats: map[string]HostStats{
				"host1": {Ok: 2, Failures: 0, Rescued: 1},
			},
		},
	}

	merged := AggregateStats(output)
	h1 := merged["host1"]
	if h1.Ok != 7 || h1.Failures != 1 || h1.Changed != 2 || h1.Rescued != 1 {
		t.Errorf("host1 = %+v", h1)
	}
	h2 := merged["host2"]
	if h2.Ok != 3 || h2.Skipped != 1 {
		t.Errorf("host2 = %+v", h2)
	}
}

func TestAggregateStats_Empty(t *testing.T) {
	merged := AggregateStats(nil)
	if len(merged) != 0 {
		t.Errorf("expected empty map, got %v", merged)
	}
}

func TestExtractFailedTasks(t *testing.T) {
	output := []PlaybookOutput{
		{
			Playbook: "site.yml",
			Plays: []PlayOutput{
				{
					Tasks: []TaskOutput{
						{
							Task: map[string]any{"name": "compile"},
							Hosts: map[string]TaskHostResult{
								"host1": {Failed: true, Action: "shell", Msg: "exit code 1"},
							},
						},
						{
							Task: map[string]any{"name": "ignored-task"},
							Hosts: map[string]TaskHostResult{
								"host1": {Failed: true, IgnoreErrors: true},
							},
						},
					},
				},
			},
		},
	}
	stats := map[string]HostStats{
		"host1": {Failures: 1},
	}

	failed := ExtractFailedTasks(output, stats)
	if len(failed) != 1 {
		t.Fatalf("got %d failed tasks, want 1", len(failed))
	}
	if failed[0].Task != "compile" {
		t.Errorf("task = %q, want compile", failed[0].Task)
	}
	if failed[0].Msg != "exit code 1" {
		t.Errorf("msg = %q", failed[0].Msg)
	}
}

func TestExtractFailedTasks_SkipsRescuedHosts(t *testing.T) {
	output := []PlaybookOutput{
		{
			Plays: []PlayOutput{
				{
					Tasks: []TaskOutput{
						{
							Task: map[string]any{"name": "rescued-task"},
							Hosts: map[string]TaskHostResult{
								"host1": {Failed: true},
							},
						},
					},
				},
			},
		},
	}
	stats := map[string]HostStats{
		"host1": {Failures: 0, Rescued: 1},
	}

	failed := ExtractFailedTasks(output, stats)
	if len(failed) != 0 {
		t.Errorf("expected 0 failed tasks for rescued host, got %d", len(failed))
	}
}

func TestGetManifestURL(t *testing.T) {
	build := &Build{
		Artifacts: []Artifact{
			{Name: "docs", URL: "https://example.com/docs"},
			{Name: "logs", URL: "https://example.com/logs", Metadata: map[string]any{"type": "zuul_manifest"}},
		},
	}
	if got := GetManifestURL(build); got != "https://example.com/logs" {
		t.Errorf("got %q", got)
	}
}

func TestGetManifestURL_NotFound(t *testing.T) {
	build := &Build{
		Artifacts: []Artifact{
			{Name: "docs", URL: "https://example.com/docs"},
		},
	}
	if got := GetManifestURL(build); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCollectFiles(t *testing.T) {
	nodes := []ManifestNode{
		{
			Name: "logs",
			Children: []ManifestNode{
				{Name: "console.log", Size: 1024},
				{Name: "job-output.json", Size: 512},
			},
		},
		{Name: "zuul-info", Children: []ManifestNode{
			{Name: "inventory.yaml", Size: 256},
		}},
	}

	files := CollectFiles(nodes)
	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}

	paths := map[string]int64{}
	for _, f := range files {
		paths[f.Path] = f.Size
	}
	if paths["/logs/console.log"] != 1024 {
		t.Error("missing /logs/console.log")
	}
	if paths["/logs/job-output.json"] != 512 {
		t.Error("missing /logs/job-output.json")
	}
	if paths["/zuul-info/inventory.yaml"] != 256 {
		t.Error("missing /zuul-info/inventory.yaml")
	}
}

func TestCollectFiles_Empty(t *testing.T) {
	files := CollectFiles(nil)
	if len(files) != 0 {
		t.Errorf("expected empty, got %d", len(files))
	}
}

func TestAnyToString(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{"string", "hello", "hello"},
		{"nil", nil, ""},
		{"slice", []any{"a", "b"}, "a\nb"},
		{"number", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := anyToString(tt.val); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCmdString(t *testing.T) {
	tests := []struct {
		name string
		cmd  any
		want string
	}{
		{"string", "ls -la", "ls -la"},
		{"slice", []any{"ls", "-la"}, "ls -la"},
		{"nil", nil, ""},
		{"number", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := TaskHostResult{Cmd: tt.cmd}
			if got := r.CmdString(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMsgString(t *testing.T) {
	r := TaskHostResult{Msg: "something failed"}
	if got := r.MsgString(); got != "something failed" {
		t.Errorf("got %q", got)
	}
}

func TestExtractFailedTasks_UsesSterrAsFallback(t *testing.T) {
	output := []PlaybookOutput{
		{
			Plays: []PlayOutput{{
				Tasks: []TaskOutput{{
					Task:  map[string]any{"name": "t"},
					Hosts: map[string]TaskHostResult{"h1": {Failed: true, Stderr: "stderr msg"}},
				}},
			}},
		},
	}
	stats := map[string]HostStats{"h1": {Failures: 1}}
	failed := ExtractFailedTasks(output, stats)
	if len(failed) != 1 || failed[0].Msg != "stderr msg" {
		t.Errorf("expected stderr fallback, got %+v", failed)
	}
}

func TestProjectBestName(t *testing.T) {
	tests := []struct {
		name string
		p    Project
		want string
	}{
		{"canonical", Project{CanonicalName: "org/repo"}, "org/repo"},
		{"project canonical", Project{ProjectCanonicalName: "org/repo2"}, "org/repo2"},
		{"connection+name", Project{ConnectionName: "github", Name: "repo"}, "github/repo"},
		{"name only", Project{Name: "repo"}, "repo"},
		{"empty", Project{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.BestName(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeTypeString(t *testing.T) {
	tests := []struct {
		name string
		typ  any
		want string
	}{
		{"string", "ubuntu-focal", "ubuntu-focal"},
		{"slice", []any{"ubuntu", "focal"}, "ubuntu, focal"},
		{"nil", nil, ""},
		{"number", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := Node{Type: tt.typ}
			if got := n.TypeString(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeDisplayLabel(t *testing.T) {
	if got := (Node{Label: "my-label"}).DisplayLabel(); got != "my-label" {
		t.Errorf("got %q", got)
	}
	if got := (Node{Type: "ubuntu"}).DisplayLabel(); got != "ubuntu" {
		t.Errorf("got %q", got)
	}
}

func TestAutoholdHoldDuration(t *testing.T) {
	tests := []struct {
		expiry int
		want   string
	}{
		{0, "-"},
		{-1, "-"},
		{90000, "1d 1h"},
		{7200, "2h 0m"},
		{300, "5m"},
	}
	for _, tt := range tests {
		h := Autohold{NodeExpiry: tt.expiry}
		if got := h.HoldDuration(); got != tt.want {
			t.Errorf("HoldDuration(%d) = %q, want %q", tt.expiry, got, tt.want)
		}
	}
}

func TestQueueItemProjectName(t *testing.T) {
	tests := []struct {
		name string
		item *QueueItem
		want string
	}{
		{"from refs", &QueueItem{Refs: []QueueRef{{Project: "org/repo"}}}, "org/repo"},
		{"string project", &QueueItem{Project: "fallback"}, "fallback"},
		{"map canonical", &QueueItem{Project: map[string]any{"canonical_name": "org/x"}}, "org/x"},
		{"map name", &QueueItem{Project: map[string]any{"name": "x"}}, "x"},
		{"nil project no refs", &QueueItem{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.ProjectName(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQueueItemChangeID(t *testing.T) {
	tests := []struct {
		name string
		item *QueueItem
		want string
	}{
		{"MR style", &QueueItem{Refs: []QueueRef{{ID: "215,abc123"}}}, "215"},
		{"plain id", &QueueItem{Refs: []QueueRef{{ID: "999"}}}, "999"},
		{"fallback", &QueueItem{ID: 42}, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.item.ChangeID(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQueueItemOwner(t *testing.T) {
	q := QueueItem{Refs: []QueueRef{{Owner: "alice"}}}
	if got := q.Owner(); got != "alice" {
		t.Errorf("got %q", got)
	}
	empty := &QueueItem{}
	if got := empty.Owner(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestQueueItemRefName(t *testing.T) {
	q := QueueItem{Refs: []QueueRef{{Ref: "refs/heads/main"}}}
	if got := q.RefName(); got != "refs/heads/main" {
		t.Errorf("got %q", got)
	}
	empty2 := &QueueItem{}
	if got := empty2.RefName(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestQueueItemChangeURL(t *testing.T) {
	q := QueueItem{Refs: []QueueRef{{URL: "https://review.example.com/123"}}}
	if got := q.ChangeURL(); got != "https://review.example.com/123" {
		t.Errorf("got %q", got)
	}
	empty3 := &QueueItem{}
	if got := empty3.ChangeURL(); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
