package api

type Semaphore struct {
	Name  string `json:"name"`
	Max   int    `json:"max,omitempty"`
	Value int    `json:"value,omitempty"`
}

func (c *Client) GetSemaphores() ([]Semaphore, error) {
	var sems []Semaphore
	err := c.getJSON(c.tenantPath("semaphores"), nil, &sems)
	return sems, err
}
