# agentic_instructions.md

## Purpose
Datadog monitors API proxy. Lists monitors with pagination, retrieves triggered monitors, fetches individual monitors by ID, and provides monitor ID listings.

## Technology
Go, net/http, strconv

## Contents
- `handlers.go` -- ListMonitors, GetMonitorPageCount, GetTriggeredMonitors, GetMonitorByID, GetMonitorIDs
- `types.go` -- Monitor, MonitorSearchResponse, Metadata, Creator, MonitorListResponse, TriggeredMonitorsResponse

## Key Functions
- `ListMonitors(w, r) (int, any)` -- Paginated monitor list (default: page=0, per_page=30)
- `GetMonitorPageCount(w, r) (int, any)` -- Returns pagination metadata
- `GetTriggeredMonitors(w, r) (int, any)` -- Fetches all monitors, filters for Alert/Warn status
- `GetMonitorByID(w, r, idStr) (int, any)` -- Single monitor by ID via path parameter
- `GetMonitorIDs(w, r) (int, any)` -- Returns ID, name, status for each monitor

## Data Types
- `Monitor` -- struct: ID, Name, Status, Type, Query, Message, Tags, Priority, Created, Modified, Creator, OverallState
- `MonitorSearchResponse` -- struct: Monitors []Monitor, Metadata
- `Metadata` -- struct: Page, PageCount, PerPage, Total
- `Creator` -- struct: Email, Handle, Name
- `MonitorListResponse` -- struct: Monitors, Page, PageCount, PerPage, TotalCount
- `TriggeredMonitorsResponse` -- struct: Monitors, Count

## Logging
None

## CRUD Entry Points
- **Create**: N/A (read-only proxy)
- **Read**: Call `monitors.ListMonitors`, `monitors.GetTriggeredMonitors`, etc.
- **Update**: Add monitor update/mute endpoints
- **Delete**: N/A

## Style Guide
- Pagination via query params: `?page=0&per_page=30`
- Uses `urls.SearchMontiors` (note typo is intentional, matches URL package)
- Representative snippet:

```go
func GetTriggeredMonitors(w http.ResponseWriter, r *http.Request) (int, any) {
	allMonitors := []Monitor{}
	page := 0
	for {
		url := urls.SearchMontiors + "?page=" + strconv.Itoa(page) + "&per_page=100"
		result, status, err := requests.Get[MonitorSearchResponse](w, r, url)
		if err != nil {
			return http.StatusInternalServerError, map[string]string{"error": err.Error()}
		}
		allMonitors = append(allMonitors, result.Monitors...)
		if page >= result.Metadata.PageCount-1 {
			break
		}
		page++
	}
	// Filter for Alert/Warn...
}
```
