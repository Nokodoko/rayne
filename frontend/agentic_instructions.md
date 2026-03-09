# agentic_instructions.md -- frontend

## Purpose
Go-based web frontend serving the Rayne portfolio/chat UI using Templ templates, static CSS, and JavaScript for the Monty chatbot interface.

## Technology
Go 1.x, net/http, a](-h/templ templating engine, vanilla JavaScript, CSS

## Contents
- `main.go` -- HTTP server serving static files and templ-rendered pages on port 3000
- `Makefile` -- Build/run commands
- `templates/` -- Templ template files (.templ) and generated Go files (_templ.go)
- `static/css/` -- CSS stylesheets
- `static/js/` -- JavaScript files including chat.js for Monty chatbot

## Key Functions
- `main()` -- Starts HTTP server with static file handler and route handlers
- `handleIndex(w, r)` -- Renders the index page using `templates.Index()`
- `handleHealth(w, r)` -- Health check endpoint
- `cacheControlHandler(next http.Handler) http.Handler` -- Disables caching for JS files

## Data Types
None.

## Logging
`log.Printf` for server startup

## CRUD Entry Points
- **Create**: Add new template in templates/, register route in main.go
- **Read**: GET / serves index page, GET /health for health check
- **Update**: Modify .templ files, run templ generate, update static assets
- **Delete**: Remove routes from main.go

## Style Guide
- Templ templating for HTML generation
- Static files served from /static/ prefix
- Environment variables: SERVER_HOST, SERVER_PORT
- Representative snippet:
```go
func main() {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", cacheControlHandler(fs)))
	mux.HandleFunc("/", handleIndex)
	addr := host + ":" + port
	http.ListenAndServe(addr, mux)
}
```
