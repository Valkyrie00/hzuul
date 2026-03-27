package api

import "fmt"

type Autohold struct {
	ID           string  `json:"id"`
	Tenant       string  `json:"tenant,omitempty"`
	Project      string  `json:"project"`
	Job          string  `json:"job"`
	RefFilter    string  `json:"ref_filter,omitempty"`
	MaxCount     int     `json:"max_count,omitempty"`
	CurrentCount int     `json:"current_count,omitempty"`
	Reason       string  `json:"reason,omitempty"`
	NodeExpiry   int     `json:"node_expiration,omitempty"`
	Expired      float64 `json:"expired,omitempty"`
}

func (h Autohold) HoldDuration() string {
	s := h.NodeExpiry
	if s <= 0 {
		return "-"
	}
	d := s / 86400
	hr := (s % 86400) / 3600
	m := (s % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh", d, hr)
	}
	if hr > 0 {
		return fmt.Sprintf("%dh %dm", hr, m)
	}
	return fmt.Sprintf("%dm", m)
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
