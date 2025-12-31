package monitors

// MonitorSearchResponse represents the response from monitor search API
type MonitorSearchResponse struct {
	Monitors []Monitor `json:"monitors"`
	Metadata Metadata  `json:"metadata"`
}

// Monitor represents a Datadog monitor
type Monitor struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Type         string   `json:"type"`
	Query        string   `json:"query"`
	Message      string   `json:"message"`
	Tags         []string `json:"tags"`
	Priority     *int     `json:"priority,omitempty"`
	Created      string   `json:"created"`
	Modified     string   `json:"modified"`
	Creator      Creator  `json:"creator"`
	OverallState string   `json:"overall_state"`
}

// Metadata represents pagination metadata
type Metadata struct {
	Page      int `json:"page"`
	PageCount int `json:"page_count"`
	PerPage   int `json:"per_page"`
	Total     int `json:"total_count"`
}

// Creator represents the creator of a monitor
type Creator struct {
	Email  string `json:"email"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
}

// MonitorListResponse is the response format for list endpoints
type MonitorListResponse struct {
	Monitors   []Monitor `json:"monitors"`
	Page       int       `json:"page"`
	PageCount  int       `json:"page_count"`
	PerPage    int       `json:"per_page"`
	TotalCount int       `json:"total_count"`
}

// TriggeredMonitorsResponse is for triggered monitors endpoint
type TriggeredMonitorsResponse struct {
	Monitors []Monitor `json:"monitors"`
	Count    int       `json:"count"`
}
