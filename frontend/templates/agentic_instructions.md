# agentic_instructions.md

## Purpose
Templ HTML components for the portfolio frontend. Three components compose the full page: layout (base HTML), index (content), and chat (AI widget).

## Technology
Go templ (a-h/templ v0.3.977), HTMX 1.9.10, Inter font (Google Fonts)

## Contents
- `layout.templ` -- Base HTML wrapper: loads HTMX, Inter font, injects window.RAYNE_API_BASE
- `layout_templ.go` -- Generated Go code from layout.templ
- `index.templ` -- Portfolio content: wraps with @Layout("n0ko"), includes @ChatWidget()
- `index_templ.go` -- Generated Go code from index.templ
- `chat.templ` -- Floating chat widget UI: toggle button, message container, input field
- `chat_templ.go` -- Generated Go code from chat.templ

## Key Functions
- `Layout(title string) templ.Component` -- Base HTML layout with head, scripts, and body wrapper
- `Index() templ.Component` -- Main page content with portfolio sections
- `ChatWidget() templ.Component` -- Floating chat bubble with WebSocket connection to Monty gateway

## Data Types
N/A (templ components)

## Logging
N/A

## CRUD Entry Points
- **Create**: Add new .templ files, run `templ generate`, commit both .templ and _templ.go
- **Read**: Components are rendered via `component.Render(ctx, w)` in main.go
- **Update**: Edit .templ files, re-run `templ generate`, commit generated files
- **Delete**: Remove .templ and corresponding _templ.go files

## Style Guide
- Templ syntax: `templ ComponentName(args) { ... }` with `@ChildComponent()` composition
- Generated files must be committed (required for go build)
- HTMX loaded from CDN in layout
- Representative templ snippet:

```templ
templ Layout(title string) {
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<title>{ title }</title>
		<script src="https://unpkg.com/htmx.org@1.9.10"></script>
	</head>
	<body>
		{ children... }
	</body>
	</html>
}
```
