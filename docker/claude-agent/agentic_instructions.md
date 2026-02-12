# agentic_instructions.md

## Purpose
Claude AI sidecar for automated Root Cause Analysis (RCA). Node.js HTTP server that wraps Claude Code CLI, pre-fetches Datadog data, generates embeddings via Ollama, stores/searches similar incidents in Qdrant, and creates Datadog Notebooks with analysis results.

## Technology
Node.js 22, Express-style HTTP server, @anthropic-ai/claude-code, @anthropic-ai/sdk, Python 3 (dd_lib tools), Qdrant (vector DB), Ollama (embeddings)

## Contents
- `agent-server.js` -- Complete agent server (~1182 lines): HTTP endpoints, Claude Code invocation (dual auth: OAuth token or API key), Datadog data pre-fetching, Qdrant vector search, notebook generation
- `Dockerfile` -- Node 22 Alpine + Python 3 + Claude Code CLI + dd_lib, runs as non-root user on port 9000

## Key Functions
- `POST /analyze` -- Main RCA endpoint: receives webhook payload, pre-fetches Datadog data (logs, host info, events, monitor config), invokes Claude for analysis, generates embeddings, stores in Qdrant, creates Datadog Notebook
- `POST /generate-notebook` -- Generates Datadog notebook from analysis results
- `POST /tools/execute` -- Executes dd_lib Python tools
- `POST /tools/create-function` -- Creates new dd_lib tool functions
- `GET /tools` -- Lists available dd_lib tools
- `GET /health` -- Health check
- `invokeClaudeCode(prompt, workDir)` -- Invokes Claude Code CLI with dual auth support
- `createDatadogNotebook(monitorId, analysis, data)` -- Creates Datadog API v1 notebook
- `generateEmbeddings(text)` -- Generates vector embeddings via Ollama
- `storeRCA(monitorId, analysis, embedding)` -- Stores RCA in Qdrant
- `searchSimilarRCAs(embedding, limit)` -- Finds similar past incidents

## Data Types
- Analyze request: `{payload: {monitor_id, monitor_name, alert_status, hostname, ...}, credentials: {api_key, app_key, base_url}}`
- Analyze response: `{success, monitorId, analysis, notebook: {url}, timestamp}`
- Tools: dd_lib Python functions (get_hosts, get_events, search_logs, etc.)

## Logging
Uses `console.log`, `console.error` with timestamps

## CRUD Entry Points
- **Create**: POST /tools/create-function to add new dd_lib tools
- **Read**: GET /tools lists available tools, POST /analyze runs analysis
- **Update**: Modify SYSTEM_PROMPT, Qdrant collection config, notebook template
- **Delete**: N/A

## Style Guide
- Dual auth: prefers ANTHROPIC_OAUTH_TOKEN over ANTHROPIC_API_KEY
- Pre-fetch pattern: gather Datadog context before invoking Claude
- Vector similarity search for incident deduplication
- Representative Dockerfile snippet:

```dockerfile
FROM node:22-alpine
RUN apk add --no-cache python3 py3-pip git curl bash
COPY dd_lib/dd_lib/ /app/dd_lib/
RUN npm install -g @anthropic-ai/claude-code && npm install -g @anthropic-ai/sdk
COPY docker/claude-agent/agent-server.js /app/
EXPOSE 9000
CMD ["node", "/app/agent-server.js"]
```
