# agentic_instructions.md

## Purpose
Datadog hosts API proxy. Retrieves host lists, active host counts, host tags (per-host and all-hosts), and provides helper functions for other packages.

## Technology
Go, net/http, encoding/json, io

## Contents
- `getHosts.go` -- GetHosts handler and GetHostsHelper internal function
- `getActivehosts.go` -- GetTotalActiveHosts handler
- `getHostTags.go` -- GetHostTags (direct API call), GetHostTagsHandler, GetAllHostsTags
- `hostResponse.go` -- HostListResponse type (detailed host metadata)
- `activeHostResponse.go` -- ActiveHostsResponse type

## Key Functions
- `GetHosts(w, r) (int, any)` -- Returns list of hostnames from Datadog
- `GetHostsHelper(w, r) []string` -- Internal helper returning hostname list (used by GetAllHostsTags)
- `GetTotalActiveHosts(w, r) (int, any)` -- Returns active/up host counts
- `GetHostTags(hostname string) ([]string, error)` -- Fetches tags for a specific host (direct HTTP call)
- `GetHostTagsHandler(w, r, hostname) (int, any)` -- HTTP handler wrapper for GetHostTags
- `GetAllHostsTags(w, r) (int, any)` -- Fetches tags for all hosts (iterates GetHostsHelper results)

## Data Types
- `HostListResponse` -- struct: HostList[]{Aliases, Apps, HostName, Meta{AgentVersion, CpuCores, Platform}, Metrics{CPU, Load}, TagsBySource}, TotalMatching, TotalReturned
- `ActiveHostsResponse` -- struct: Total_active, Total_up
- `HostTagsResponse` -- struct: Tags []string

## Logging
Uses `log.Printf` in GetTotalActiveHosts

## CRUD Entry Points
- **Create**: N/A (read-only proxy)
- **Read**: Import and call `hosts.GetHosts`, `hosts.GetHostTags`, etc.
- **Update**: Add new host-related endpoints or response field extraction
- **Delete**: N/A

## Style Guide
- Mixed patterns: GetHostTags uses direct http.Client, GetHosts uses requests.Get[T]
- Helper functions exposed for cross-package use (GetHostsHelper)
- Representative snippet:

```go
func GetHosts(w http.ResponseWriter, r *http.Request) (int, any) {
	hosts, status, err := requests.Get[HostListResponse](w, r, urls.GetHosts)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	var listHosts []string
	for _, v := range hosts.HostList {
		listHosts = append(listHosts, v.HostName)
	}
	return status, listHosts
}
```
