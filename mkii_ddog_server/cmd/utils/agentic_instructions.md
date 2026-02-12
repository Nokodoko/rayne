# agentic_instructions.md

## Purpose
HTTP handler utilities -- endpoint registration helpers, JSON parsing/writing, environment variable reader.

## Technology
Go, net/http, encoding/json

## Contents
- `utils.go` -- Endpoint(), EndpointWithPathParams(), GetEnv(), ParseJson(), WriteJson(), WriteError()

## Key Functions
- `Endpoint(router *http.ServeMux, method string, path string, endpt func(w, r) (int, any))` -- Registers a handler using Go 1.22+ method routing, sets JSON content type, encodes response
- `EndpointWithPathParams(router, method, path, val string, endpt func(w, r, pv string) (int, any))` -- Same but extracts a path parameter via `r.PathValue(val)`
- `GetEnv(key string, fallback string) string` -- Reads env var with default
- `ParseJson[T any](r *http.Request, payload *T) (int, any, error)` -- Generic JSON body decoder
- `WriteJson[T any](w http.ResponseWriter, status int, data T) error` -- Generic JSON response encoder
- `WriteError(w http.ResponseWriter, status int, v error)` -- Error response helper

## Data Types
None

## Logging
Uses `log.Printf` with `[ERROR]` prefix for 4xx+ responses.

## CRUD Entry Points
- **Create**: Add new utility functions to utils.go
- **Read**: Import `github.com/Nokodoko/mkii_ddog_server/cmd/utils`
- **Update**: Modify existing functions
- **Delete**: Remove functions (check callers)

## Style Guide
- Handler signature convention: `func(w http.ResponseWriter, r *http.Request) (int, any)`
- Go generics used for ParseJson and WriteJson
- Representative snippet:

```go
func Endpoint(router *http.ServeMux, method string, path string, endpt func(w http.ResponseWriter, r *http.Request) (int, any)) {
	pattern := method + " " + path
	router.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status, resp := endpt(w, r)
		if status >= 400 {
			log.Printf("[ERROR] %s %s returned status %d", r.Method, r.URL.Path, status)
		}
		if resp != nil {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(resp)
		}
	})
}
```
