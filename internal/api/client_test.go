package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(handler http.Handler) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return &Client{
		baseURL: srv.URL,
		tenant:  "test",
		doer:    srv.Client(),
	}, srv
}

func TestGet_SendsCorrectRequest(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	params := map[string][]string{"limit": {"10"}, "skip": {"5"}}
	resp, err := c.get("/api/tenant/test/builds", params)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}
	if gotPath != "/api/tenant/test/builds" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "limit=10") || !strings.Contains(gotQuery, "skip=5") {
		t.Errorf("query = %q, missing expected params", gotQuery)
	}
}

func TestGet_Returns4xxError(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("resource not found"))
	}))
	defer srv.Close()

	_, err := c.get("/missing", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should contain status code: %v", err)
	}
	if !strings.Contains(err.Error(), "resource not found") {
		t.Errorf("error should contain body: %v", err)
	}
}

func TestGet_Returns5xxError(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		w.Write([]byte("overloaded"))
	}))
	defer srv.Close()

	_, err := c.get("/fail", nil)
	if err == nil {
		t.Fatal("expected error for 503")
	}
}

func TestGetJSON_DecodesResponse(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"uuid":"abc","job_name":"tox","result":"FAILURE"}]`))
	}))
	defer srv.Close()

	var builds []Build
	err := c.getJSON("/builds", nil, &builds)
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 1 {
		t.Fatalf("got %d builds, want 1", len(builds))
	}
	if builds[0].UUID != "abc" || builds[0].JobName != "tox" || builds[0].Result != "FAILURE" {
		t.Errorf("decoded incorrectly: %+v", builds[0])
	}
}

func TestGetJSON_PropagatesHTTPError(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	var result map[string]string
	err := c.getJSON("/fail", nil, &result)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestPostJSON_SendsCorrectRequest(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var gotBody map[string]string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := c.postJSON("/api/tenant/test/project/nova/enqueue", map[string]string{"pipeline": "gate"})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/api/tenant/test/project/nova/enqueue" {
		t.Errorf("path = %q", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q", gotContentType)
	}
	if gotBody["pipeline"] != "gate" {
		t.Errorf("body = %v", gotBody)
	}
}

func TestPostJSON_ErrorWithBody(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte("validation failed"))
	}))
	defer srv.Close()

	err := c.postJSON("/action", map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should contain body: %v", err)
	}
}

func TestPostJSON_ErrorEmptyBody(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	err := c.postJSON("/action", map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestDelete_SendsCorrectRequest(t *testing.T) {
	var gotMethod, gotPath string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	err := c.delete("/api/tenant/test/autohold/h1")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/api/tenant/test/autohold/h1" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestDelete_ReturnsErrorOn4xx(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	err := c.delete("/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status: %v", err)
	}
}

func TestRawGet_ReturnsFullBody(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("line1\nline2\nline3"))
	}))
	defer srv.Close()

	resp, err := c.RawGet(srv.URL + "/logs/console.log")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "line1\nline2\nline3" {
		t.Errorf("body = %q", body)
	}
}

func TestRawGet_ErrorIncludesURLAndBody(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("access denied"))
	}))
	defer srv.Close()

	_, err := c.RawGet(srv.URL + "/secret")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error = %v", err)
	}
}

func TestFetchFileContent_ReturnsExactContent(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("2024-01-01 ERROR: something broke\n2024-01-01 FATAL: giving up"))
	}))
	defer srv.Close()

	content, err := c.FetchFileContent(srv.URL+"/log.txt", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "FATAL: giving up") {
		t.Errorf("content = %q", content)
	}
}

func TestFetchFileContent_RespectsLimit(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("0123456789abcdef"))
	}))
	defer srv.Close()

	content, err := c.FetchFileContent(srv.URL+"/big.log", 5)
	if err != nil {
		t.Fatal(err)
	}
	if content != "01234" {
		t.Errorf("content = %q, want exactly '01234'", content)
	}
}

func TestGetBuilds_VerifiesPathAndFilter(t *testing.T) {
	var gotPath, gotQuery string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`[{"uuid":"b1","job_name":"tox","result":"SUCCESS","pipeline":"check"}]`))
	}))
	defer srv.Close()

	builds, err := c.GetBuilds(&BuildFilter{JobName: "tox", Pipeline: "check", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/tenant/test/builds" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "job_name=tox") {
		t.Errorf("missing job_name param: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "pipeline=check") {
		t.Errorf("missing pipeline param: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "limit=10") {
		t.Errorf("missing limit param: %q", gotQuery)
	}
	if len(builds) != 1 || builds[0].UUID != "b1" || builds[0].Pipeline != "check" {
		t.Errorf("builds = %+v", builds)
	}
}

func TestGetBuild_VerifiesPath(t *testing.T) {
	var gotPath string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"uuid":"xyz","job_name":"deploy","result":"FAILURE"}`))
	}))
	defer srv.Close()

	build, err := c.GetBuild("xyz")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/tenant/test/build/xyz" {
		t.Errorf("path = %q", gotPath)
	}
	if build.UUID != "xyz" || build.Result != "FAILURE" {
		t.Errorf("build = %+v", build)
	}
}

func TestEnqueue_SendsCorrectPayload(t *testing.T) {
	var gotPath string
	var gotPayload EnqueueRequest
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotPayload)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := c.Enqueue("openstack/nova", &EnqueueRequest{Pipeline: "gate", Change: "123,1"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/tenant/test/project/openstack/nova/enqueue" {
		t.Errorf("path = %q", gotPath)
	}
	if gotPayload.Pipeline != "gate" || gotPayload.Change != "123,1" {
		t.Errorf("payload = %+v", gotPayload)
	}
}

func TestDequeue_SendsCorrectPayload(t *testing.T) {
	var gotPayload DequeueRequest
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotPayload)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := c.Dequeue("openstack/nova", &DequeueRequest{Pipeline: "check", Project: "openstack/nova"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPayload.Pipeline != "check" || gotPayload.Project != "openstack/nova" {
		t.Errorf("payload = %+v", gotPayload)
	}
}

func TestPromote_SendsCorrectPayload(t *testing.T) {
	var gotPayload PromoteRequest
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotPayload)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := c.Promote(&PromoteRequest{Pipeline: "gate", Changes: []string{"123,1", "456,2"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotPayload.Pipeline != "gate" || len(gotPayload.Changes) != 2 {
		t.Errorf("payload = %+v", gotPayload)
	}
}

func TestDeleteAutohold_VerifiesMethodAndPath(t *testing.T) {
	var gotMethod, gotPath string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()

	err := c.DeleteAutohold("hold-abc")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Errorf("method = %q", gotMethod)
	}
	if gotPath != "/api/tenant/test/autohold/hold-abc" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestGetJobOutput_VerifiesURL(t *testing.T) {
	var gotPath string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`[{"playbook":"site.yml","phase":"run","stats":{"h1":{"failures":1}}}]`))
	}))
	defer srv.Close()

	output, err := c.GetJobOutput(srv.URL + "/logs/")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/logs/job-output.json" {
		t.Errorf("path = %q, want /logs/job-output.json", gotPath)
	}
	if len(output) != 1 || output[0].Playbook != "site.yml" || output[0].Phase != "run" {
		t.Errorf("output = %+v", output)
	}
	if output[0].Stats["h1"].Failures != 1 {
		t.Errorf("stats not decoded: %+v", output[0].Stats)
	}
}

func TestGetJobOutput_ServerError(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	_, err := c.GetJobOutput(srv.URL + "/logs/")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestFetchManifest_DecodesTree(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tree":[{"name":"logs","children":[{"name":"console.log","size":4096}]}]}`))
	}))
	defer srv.Close()

	tree, err := c.FetchManifest(srv.URL + "/manifest")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Tree) != 1 {
		t.Fatalf("tree has %d roots, want 1", len(tree.Tree))
	}
	if tree.Tree[0].Name != "logs" || len(tree.Tree[0].Children) != 1 {
		t.Errorf("tree = %+v", tree.Tree[0])
	}
	if tree.Tree[0].Children[0].Name != "console.log" || tree.Tree[0].Children[0].Size != 4096 {
		t.Errorf("child = %+v", tree.Tree[0].Children[0])
	}
}

func TestGetStatus_DecodesStructure(t *testing.T) {
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"pipelines":[{"name":"check","change_queues":[{"name":"q1"}]}]}`))
	}))
	defer srv.Close()

	status, err := c.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Pipelines) != 1 || status.Pipelines[0].Name != "check" {
		t.Errorf("pipelines = %+v", status.Pipelines)
	}
}

func TestGetSystemEvents_PassesParams(t *testing.T) {
	var gotQuery string
	c, srv := testClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	_, err := c.GetSystemEvents(25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "limit=25") {
		t.Errorf("query = %q, missing limit", gotQuery)
	}
}

func TestHasAdminToken_NoAuth(t *testing.T) {
	c := &Client{authProvider: nil}
	if c.HasAdminToken() {
		t.Error("expected false with nil authProvider")
	}
}

func TestClientURLBuilders(t *testing.T) {
	c := &Client{baseURL: "https://zuul.example.com", tenant: "myten"}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"BuildURL", c.BuildURL("abc"), "https://zuul.example.com/t/myten/build/abc"},
		{"ProjectURL", c.ProjectURL("org/repo"), "https://zuul.example.com/t/myten/project/org/repo"},
		{"JobURL", c.JobURL("tox-py312"), "https://zuul.example.com/t/myten/job/tox-py312"},
		{"tenantPath", c.tenantPath("builds"), "/api/tenant/myten/builds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestClientHost(t *testing.T) {
	tests := []struct {
		baseURL string
		want    string
	}{
		{"https://zuul.example.com", "zuul.example.com"},
		{"https://zuul.example.com:8080", "zuul.example.com:8080"},
		{"://invalid", ""},
	}
	for _, tt := range tests {
		c := &Client{baseURL: tt.baseURL}
		if got := c.Host(); got != tt.want {
			t.Errorf("Host(%q) = %q, want %q", tt.baseURL, got, tt.want)
		}
	}
}

func TestClientTenantOps(t *testing.T) {
	c := &Client{tenant: "old"}
	if c.Tenant() != "old" {
		t.Fatalf("initial tenant = %q", c.Tenant())
	}
	c.SetTenant("new")
	if c.Tenant() != "new" {
		t.Fatalf("after SetTenant = %q", c.Tenant())
	}
	if c.tenantPath("builds") != "/api/tenant/new/builds" {
		t.Errorf("tenantPath not updated: %q", c.tenantPath("builds"))
	}
}
