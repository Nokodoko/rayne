# agentic_instructions.md

## Purpose
Generic HTTP request helpers for Datadog API calls with automatic authentication and APM tracing. Supports both default credentials and per-account credentials.

## Technology
Go generics, net/http, dd-trace-go httptrace, encoding/json

## Contents
- `requests.go` -- Get, Post, Put, Delete (default creds) + GetWithCreds, PostWithCreds, PutWithCreds, DeleteWithCreds (account-specific creds)

## Key Functions
- `Get[T any](w, r, url) (T, int, error)` -- GET with default Datadog auth headers
- `Post[T any](w, r, url, payload) (T, int, error)` -- POST with default auth
- `Put[T any](w, r, url, payload) (T, int, error)` -- PUT with default auth
- `Delete[T any](w, r, url) (T, int, error)` -- DELETE with default auth
- `GetWithCreds[T any](ctx, url, creds) (T, int, error)` -- GET with account-specific credentials
- `PostWithCreds[T any](ctx, url, payload, creds) (T, int, error)` -- POST with account-specific credentials
- `PutWithCreds[T any](ctx, url, payload, creds) (T, int, error)` -- PUT with account-specific credentials
- `DeleteWithCreds[T any](ctx, url, creds) (T, int, error)` -- DELETE with account-specific credentials

## Data Types
None (uses generics for response type T)

## Logging
Uses `log.Printf` for request creation errors and parsing failures.

## CRUD Entry Points
- **Create**: Add new HTTP method helpers following the existing pattern
- **Read**: Import `requests.Get[YourType](w, r, url)`
- **Update**: Modify shared tracedClient or header setup
- **Delete**: Remove unused method helpers

## Style Guide
- Generic functions with `[T any]` type parameter
- Zero value workaround: `zero := *new(T)` for generic return types
- Context propagation via `http.NewRequestWithContext(r.Context(), ...)`
- Error wrapping with `fmt.Errorf("...: %w", err)` for WithCreds variants
- Representative snippet:

```go
func Get[T any](w http.ResponseWriter, r *http.Request, url string) (T, int, error) {
	var parsedResponse T
	zero := *new(T)
	req, err := http.NewRequestWithContext(r.Context(), "GET", url, bytes.NewBufferString(""))
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())
	response, err := tracedClient.Do(req)
	// ...
}
```
