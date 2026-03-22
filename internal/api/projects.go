package api

type Project struct {
	Name             string `json:"name"`
	Type             string `json:"type,omitempty"`
	CanonicalName    string `json:"canonical_name,omitempty"`
	ConnectionName   string `json:"connection_name,omitempty"`
}

func (c *Client) GetProjects() ([]Project, error) {
	var projects []Project
	err := c.getJSON(c.tenantPath("projects"), nil, &projects)
	return projects, err
}

func (c *Client) GetProject(name string) (map[string]any, error) {
	var result map[string]any
	err := c.getJSON(c.tenantPath("project/"+name), nil, &result)
	return result, err
}
