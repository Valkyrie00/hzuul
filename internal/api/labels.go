package api

type Label struct {
	Name string `json:"name"`
}

func (c *Client) GetLabels() ([]Label, error) {
	var labels []Label
	err := c.getJSON(c.tenantPath("labels"), nil, &labels)
	return labels, err
}
