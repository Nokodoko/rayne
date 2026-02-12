# agentic_instructions.md

## Purpose
Datadog events API proxy. Retrieves events and extracts message content.

## Technology
Go, net/http

## Contents
- `getEvents.go` -- GetEvents handler
- `eventResponse.go` -- EventsResponse type (deeply nested Datadog API response)

## Key Functions
- `GetEvents(w, r) (int, any)` -- Fetches events from Datadog API, returns message strings

## Data Types
- `EventsResponse` -- struct: deeply nested Data[].Attributes.Message, includes monitor metadata, pagination (Links.Next, Meta)

## Logging
None

## CRUD Entry Points
- **Create**: N/A (read-only proxy)
- **Read**: Call `GetEvents(w, r)` for Datadog event messages
- **Update**: Modify EventsResponse to capture additional fields
- **Delete**: N/A

## Style Guide
- Extracts only message strings from the full Datadog response
- Representative snippet:

```go
func GetEvents(w http.ResponseWriter, r *http.Request) (int, any) {
	eventrequest, status, err := requests.Get[EventsResponse](w, r, urls.GetEvents)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	var events []string
	for _, v := range eventrequest.Data {
		events = append(events, v.Attributes.Message)
	}
	return status, events
}
```
