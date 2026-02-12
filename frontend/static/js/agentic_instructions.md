# agentic_instructions.md

## Purpose
Client-side JavaScript for the portfolio frontend. Handles the Monty AI chatbot WebSocket connection and Datadog RUM visitor tracking.

## Technology
Vanilla JavaScript, WebSocket API, localStorage, Datadog RUM SDK

## Contents
- `chat.js` -- WebSocket client for Monty chatbot gateway. Protocol-aware: wss://gateway.n0kos.com/chat/ws over HTTPS, ws://192.168.50.179:8001/chat/ws over HTTP. Reconnects up to 5 times with 3s delay.
- `datadog-rum-init.js` -- RUM visitor tracking. Calls backend /v1/rum/init, /v1/rum/track, /v1/rum/session/end. Stores visitor UUID and session in localStorage.

## Key Functions
- `chat.js`: WebSocket connect, send message, receive streamed tokens, reconnect logic
- `datadog-rum-init.js`: initVisitor(), trackPageView(), trackAction(), endSession()

## Data Types
N/A (JavaScript)

## Logging
Console.log for debug, console.error for failures

## CRUD Entry Points
- **Create**: Add new .js files in this directory, reference in templ templates
- **Read**: Files served via /static/js/ with no-cache headers
- **Update**: Edit JS files directly (no build step), commit changes
- **Delete**: Remove JS file and references in templ templates

## Style Guide
- Protocol detection: `window.location.protocol === 'https:'` to choose ws:// vs wss://
- localStorage for visitor persistence across page loads
- Datadog RUM site: ddog-gov.datadoghq.com (US Gov cloud)
