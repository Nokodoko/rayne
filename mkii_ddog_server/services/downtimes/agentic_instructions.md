# agentic_instructions.md

## Purpose
Datadog downtimes API proxy. Retrieves active downtimes (monitor IDs and scopes) and provides a stub for creating downtimes.

## Technology
Go, net/http, encoding/json

## Contents
- `getDowntimes.go` -- GetDowntimes handler, Must generic helper
- `downtimesResponse.go` -- DowntimesResponse and combinedData types
- `addDowntimes.go` -- CreateDowntimes stub (log.Fatal placeholder)

## Key Functions
- `GetDowntimes(w, r) (int, any)` -- Fetches downtimes from Datadog API, extracts monitor IDs and scopes
- `Must[T any](x T, err error) T` -- Generic assert helper (log.Fatal on error)
- `CreateDowntimes() error` -- Stub (log.Fatal, not implemented)

## Data Types
- `DowntimesResponse` -- struct: Data[].Attributes.Monitor_identifier.MonitorID, Data[].Attributes.Scope
- `combinedData` -- struct: IDs []int, Scopes []string

## Logging
Uses `log.Fatal` in Must helper; `fmt.Println` for debug output

## CRUD Entry Points
- **Create**: Implement `CreateDowntimes()` (currently a stub)
- **Read**: Call `GetDowntimes(w, r)` for current downtimes
- **Update**: N/A
- **Delete**: N/A

## Style Guide
- Early-stage code with debug `fmt.Println` statements
- Uses generic `requests.Get[T]` pattern for Datadog API calls
- Representative snippet:

```go
func GetDowntimes(w http.ResponseWriter, r *http.Request) (int, any) {
	downtimes, _, err := requests.Get[DowntimesResponse](w, r, urls.GetDowntimesUrl)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err)
	}

	ids := []int{}
	scopes := []string{}
	for _, v := range downtimes.Data {
		ids = append(ids, v.Attributes.Monitor_identifier.MonitorID)
		scopes = append(scopes, v.Attributes.Scope)
	}
	return http.StatusOK, combinedData{IDs: ids, Scopes: scopes}
}
```
