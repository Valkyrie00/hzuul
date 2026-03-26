package api

type Job struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Variants    []JobVariant `json:"variants,omitempty"`
}

type JobVariant struct {
	Branches []string `json:"branches,omitempty"`
	Parent   string   `json:"parent,omitempty"`
}

func (c *Client) GetJobs() ([]Job, error) {
	var jobs []Job
	err := c.getJSON(c.tenantPath("jobs"), nil, &jobs)
	return jobs, err
}

func (c *Client) GetJob(name string) ([]map[string]any, error) {
	var result []map[string]any
	err := c.getJSON(c.tenantPath("job/"+name), nil, &result)
	return result, err
}
