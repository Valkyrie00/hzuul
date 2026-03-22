package api

type Autohold struct {
	ID          string `json:"id"`
	Tenant      string `json:"tenant,omitempty"`
	Project     string `json:"project"`
	Job         string `json:"job"`
	RefFilter   string `json:"ref_filter,omitempty"`
	MaxCount    int    `json:"max_count,omitempty"`
	CurrentCount int   `json:"current_count,omitempty"`
	Reason      string `json:"reason,omitempty"`
	NodeExpiry  int    `json:"node_expiration,omitempty"`
}

func (c *Client) GetAutoholds() ([]Autohold, error) {
	var holds []Autohold
	err := c.getJSON(c.tenantPath("autohold"), nil, &holds)
	return holds, err
}

func (c *Client) GetAutohold(id string) (*Autohold, error) {
	var hold Autohold
	err := c.getJSON(c.tenantPath("autohold/"+id), nil, &hold)
	return &hold, err
}
