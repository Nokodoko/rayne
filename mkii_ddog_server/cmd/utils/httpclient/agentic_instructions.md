# agentic_instructions.md

## Purpose
Pre-configured HTTP clients with Datadog APM tracing and connection pooling for different use cases.

## Technology
Go, net/http, dd-trace-go/contrib/net/http (httptrace)

## Contents
- `client.go` -- Five shared HTTP clients: DefaultClient, AgentClient, NotifyClient, ForwardingClient, DatadogClient

## Key Functions
None (package-level variables only)

## Data Types
- `DefaultClient` -- 30s timeout, 100 max idle conns, 10 per host
- `AgentClient` -- 180s timeout (3 min for Claude analysis), 20 max idle conns, 5 per host
- `NotifyClient` -- 5s timeout, 50 max idle conns
- `ForwardingClient` -- 10s timeout, 50 max idle conns
- `DatadogClient` -- 30s timeout, 50 max idle conns

## Logging
None

## CRUD Entry Points
- **Create**: Add a new `var XyzClient = httptrace.WrapClient(...)` declaration
- **Read**: Import `httpclient.DefaultClient` etc.
- **Update**: Modify timeout or connection pool settings
- **Delete**: Remove client variable (check for usages)

## Style Guide
- All clients wrapped with `httptrace.WrapClient` for APM visibility
- Each client has descriptive comment explaining its use case
- Representative snippet:

```go
var AgentClient = httptrace.WrapClient(&http.Client{
	Timeout: 180 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxConnsPerHost:     5,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
})
```
