package api

type EnqueueRequest struct {
	Pipeline string `json:"pipeline"`
	Project  string `json:"project"`
	Change   string `json:"change,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

type DequeueRequest struct {
	Pipeline string `json:"pipeline"`
	Project  string `json:"project"`
	Change   string `json:"change,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

type PromoteRequest struct {
	Pipeline string   `json:"pipeline"`
	Changes  []string `json:"changes"`
}

type AutoholdRequest struct {
	Job            string  `json:"job"`
	Change         *string `json:"change"`
	Ref            *string `json:"ref"`
	Reason         string  `json:"reason"`
	Count          int     `json:"count"`
	NodeHoldExpiry int     `json:"node_hold_expiration"`
}

func (c *Client) Enqueue(project string, req *EnqueueRequest) error {
	return c.postJSON(c.tenantPath("project/"+project+"/enqueue"), req)
}

func (c *Client) Dequeue(project string, req *DequeueRequest) error {
	return c.postJSON(c.tenantPath("project/"+project+"/dequeue"), req)
}

func (c *Client) Promote(req *PromoteRequest) error {
	return c.postJSON(c.tenantPath("promote"), req)
}

func (c *Client) CreateAutohold(project string, req *AutoholdRequest) error {
	return c.postJSON(c.tenantPath("project/"+project+"/autohold"), req)
}

func (c *Client) DeleteAutohold(id string) error {
	return c.delete(c.tenantPath("autohold/" + id))
}
