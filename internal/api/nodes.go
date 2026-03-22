package api

type Node struct {
	ID         string `json:"id"`
	Label      string `json:"label,omitempty"`
	Connection string `json:"connection,omitempty"`
	Type       string `json:"type,omitempty"`
	Provider   string `json:"provider,omitempty"`
	State      string `json:"state,omitempty"`
}

func (c *Client) GetNodes() ([]Node, error) {
	var nodes []Node
	err := c.getJSON(c.tenantPath("nodes"), nil, &nodes)
	return nodes, err
}
