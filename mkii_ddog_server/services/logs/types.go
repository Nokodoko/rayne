package logs

// LogSearchRequest represents a log search request
type LogSearchRequest struct {
	Filter LogFilter `json:"filter"`
	Sort   string    `json:"sort,omitempty"`
	Page   LogPage   `json:"page,omitempty"`
}

// LogFilter represents the filter for log search
type LogFilter struct {
	Query   string   `json:"query"`
	Indexes []string `json:"indexes,omitempty"`
	From    string   `json:"from"`
	To      string   `json:"to"`
}

// LogPage represents pagination options
type LogPage struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

// LogSearchResponse represents the response from log search API
type LogSearchResponse struct {
	Data  []LogEvent `json:"data"`
	Links Links      `json:"links,omitempty"`
	Meta  LogMeta    `json:"meta,omitempty"`
}

// LogEvent represents a single log event
type LogEvent struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Attributes LogEventAttrs     `json:"attributes"`
	Tags       []string          `json:"tags,omitempty"`
}

// LogEventAttrs represents log event attributes
type LogEventAttrs struct {
	Timestamp  string            `json:"timestamp"`
	Host       string            `json:"host"`
	Service    string            `json:"service"`
	Status     string            `json:"status"`
	Message    string            `json:"message"`
	Attributes map[string]any    `json:"attributes,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
}

// Links represents pagination links
type Links struct {
	Next string `json:"next,omitempty"`
}

// LogMeta represents metadata in response
type LogMeta struct {
	Page    PageInfo `json:"page,omitempty"`
	Status  string   `json:"status,omitempty"`
	Elapsed int      `json:"elapsed,omitempty"`
}

// PageInfo represents page info in metadata
type PageInfo struct {
	After string `json:"after,omitempty"`
}

// SimpleSearchRequest is a simplified search request for the API
type SimpleSearchRequest struct {
	Query string `json:"query"`
	From  string `json:"from"`
	To    string `json:"to"`
	Limit int    `json:"limit,omitempty"`
}
