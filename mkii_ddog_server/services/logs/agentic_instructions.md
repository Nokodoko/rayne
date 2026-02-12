# agentic_instructions.md

## Purpose
Datadog logs API proxy. Provides simple and advanced log search endpoints.

## Technology
Go, net/http, encoding/json

## Contents
- `handlers.go` -- SearchLogs and SearchLogsAdvanced handlers
- `types.go` -- Log search request/response types (LogSearchRequest, LogSearchResponse, LogEvent, etc.)

## Key Functions
- `SearchLogs(w, r) (int, any)` -- Simple log search with query, from, to, limit (default: 50, max: 1000)
- `SearchLogsAdvanced(w, r) (int, any)` -- Full control over Datadog log search API request

## Data Types
- `LogSearchRequest` -- struct: Filter (Query, Indexes, From, To), Sort, Page (Limit, Cursor)
- `LogFilter` -- struct: Query, Indexes, From, To
- `LogPage` -- struct: Limit, Cursor
- `LogSearchResponse` -- struct: Data []LogEvent, Links, Meta
- `LogEvent` -- struct: ID, Type, Attributes (Timestamp, Host, Service, Status, Message, Tags)
- `SimpleSearchRequest` -- struct: Query, From, To, Limit

## Logging
None

## CRUD Entry Points
- **Create**: N/A (read-only proxy)
- **Read**: POST /v1/logs/search (simple) or POST /v1/logs/search/advanced (full control)
- **Update**: Modify default indexes (currently ["main"]) or limit caps
- **Delete**: N/A

## Style Guide
- POST for search endpoints (body contains query)
- Defaults applied: query="*", limit=50, indexes=["main"]
- Representative snippet:

```go
func SearchLogs(w http.ResponseWriter, r *http.Request) (int, any) {
	var req SimpleSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()}
	}
	if req.Query == "" { req.Query = "*" }
	if req.Limit == 0 { req.Limit = 50 }

	searchReq := LogSearchRequest{
		Filter: LogFilter{Query: req.Query, Indexes: []string{"main"}, From: req.From, To: req.To},
		Page:   LogPage{Limit: req.Limit},
	}
	result, status, err := requests.Post[LogSearchResponse](w, r, urls.LogSearch, searchReq)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}
	return status, result
}
```
