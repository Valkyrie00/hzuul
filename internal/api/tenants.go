package api

type Tenant struct {
	Name     string `json:"name"`
	Projects int    `json:"projects,omitempty"`
	Queue    int    `json:"queue,omitempty"`
}

func (c *Client) GetTenants() ([]Tenant, error) {
	var tenants []Tenant
	err := c.getJSON("/api/tenants", nil, &tenants)
	return tenants, err
}
