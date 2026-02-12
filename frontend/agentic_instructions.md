# agentic_instructions.md

## Purpose
Go-based portfolio frontend serving Chris Montgomery's personal website at port 3000. Uses templ for HTML templating, HTMX for interactivity, and vanilla JS for the Monty AI chat widget and Datadog RUM tracking.

## Technology
Go, net/http, a-h/templ (v0.3.977), HTMX 1.9.10, vanilla JavaScript, CSS

## Contents
- `main.go` -- Minimal HTTP server with three routes: /, /health, /static/*
- `templates/` -- Subdirectory with templ components (layout, index, chat)
- `static/css/` -- Subdirectory with CSS styles
- `static/js/` -- Subdirectory with chat.js (WebSocket client) and datadog-rum-init.js (RUM tracking)
- `bin/` -- Build output (excluded)

## Key Functions
- `main()` -- Creates ServeMux, registers routes, starts server on SERVER_HOST:SERVER_PORT
- `handleIndex(w, r)` -- Renders templates.Index() templ component (strict "/" path match)
- `handleHealth(w, r)` -- Returns JSON health status
- `cacheControlHandler(next) http.Handler` -- Middleware: no-cache headers on .js files
- `getEnv(key, fallback) string` -- Environment variable reader with default

## Data Types
N/A

## Logging
Uses `log.Printf` for startup, `log.Fatal` for listen errors

## CRUD Entry Points
- **Create**: Add new templ components in templates/, new JS in static/js/
- **Read**: GET / serves the portfolio page
- **Update**: Edit .templ files, then run `templ generate` (must commit generated *_templ.go)
- **Delete**: Remove templ components or static files

## Style Guide
- Templ components compose via `@Layout("title")` wrapper pattern
- Static files served with cache-busting for JS
- API base URL injected as `window.RAYNE_API_BASE` in layout.templ
- WebSocket URL is protocol-dependent: wss:// for HTTPS, ws:// for HTTP
- Representative snippet:

```go
func main() {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", cacheControlHandler(fs)))
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/", handleIndex)

	host := getEnv("SERVER_HOST", "0.0.0.0")
	port := getEnv("SERVER_PORT", "3000")
	log.Printf("Server starting on http://%s:%s", host, port)
	log.Fatal(http.ListenAndServe(host+":"+port, mux))
}
```
