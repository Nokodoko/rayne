# agentic_instructions.md

## Purpose
HTTP server initialization, route registration, and middleware (CORS, APM tracing). This is the main wiring point for all API endpoints.

## Technology
Go 1.22+, net/http (stdlib ServeMux), Datadog APM (dd-trace-go)

## Contents
- `api.go` -- DDogServer struct, route registration in `Run()`, CORS middleware, APM tracing middleware, statusRecorder for response code capture

## Key Functions
- `NewDdogServer(addr string, db *sql.DB) *DDogServer` -- Constructor
- `(d *DDogServer) Run(ctx context.Context) error` -- Starts HTTP server, registers all routes, initializes storages/handlers/dispatchers, handles graceful shutdown
- `corsMiddleware(next http.Handler) http.Handler` -- Adds CORS headers, handles OPTIONS preflight
- `traceMiddleware(next http.Handler) http.Handler` -- Creates APM spans, extracts trace context from RUM SDK headers, tags errors on 4xx/5xx

## Data Types
- `DDogServer` -- struct with `addr string`, `db *sql.DB`, `dispatcher *webhooks.Dispatcher`
- `statusRecorder` -- wraps `http.ResponseWriter` to capture status code for APM tagging

## Logging
Uses `log.Printf` with bracketed prefix tags like `[APM]`. Server banner printed at startup with ASCII art and endpoint listing.

## CRUD Entry Points
- **Create**: Add new routes by calling `utils.Endpoint(router, method, path, handler)` or `utils.EndpointWithPathParams(...)` inside `Run()`
- **Read**: Inspect registered routes in the `Run()` function body
- **Update**: Modify route handlers or middleware chains in `Run()`
- **Delete**: Remove route registration lines from `Run()`

## Style Guide
- PascalCase for exported types/functions, camelCase for unexported
- Imports grouped: stdlib, then internal packages, then external (dd-trace-go)
- Error handling: log + return error; middleware uses defer/recover for panics
- Representative snippet:

```go
func (d *DDogServer) Run(ctx context.Context) error {
	router := http.NewServeMux()

	// Initialize storages
	userStorage := user.NewStorage(d.db)
	webhookStorage := webhooks.NewStorage(d.db)
	rumStorage := rum.NewStorage(d.db)
	accountStorage := accounts.NewStorage(d.db)

	// Register routes
	utils.Endpoint(router, "GET", "/health", func(w http.ResponseWriter, r *http.Request) (int, any) {
		return http.StatusOK, map[string]string{"status": "healthy"}
	})

	utils.Endpoint(router, "GET", "/v1/downtimes", downtimes.GetDowntimes)
	utils.EndpointWithPathParams(router, "GET", "/v1/hosts/{hostname}/tags", "hostname", hosts.GetHostTagsHandler)
```
