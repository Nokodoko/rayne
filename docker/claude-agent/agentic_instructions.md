# agentic_instructions.md

## Purpose
Claude AI sidecar for automated Root Cause Analysis (RCA). Node.js HTTP server that wraps Claude Code CLI, pre-fetches Datadog data, generates embeddings via Ollama, stores/searches similar incidents in Qdrant, and creates Datadog Notebooks with analysis results.

## Technology
Node.js 22, Express-style HTTP server, @anthropic-ai/claude-code, @anthropic-ai/sdk, Python 3 (dd_lib tools), Qdrant (vector DB), Ollama (embeddings)

## Contents
- `agent-server.js` -- Complete agent server (~2200 lines): HTTP endpoints, Claude Code invocation (dual auth: OAuth token or API key with auto-refresh), Datadog data pre-fetching, Qdrant vector search, notebook generation with lifecycle management (create/resolve), GoNotebook RAG integration, self-healing error recovery with exponential backoff, failure alerting (Datadog events + failure notebooks)
- `Dockerfile` -- Node 22 Alpine + Python 3 + Claude Code CLI + dd_lib, runs as non-root user on port 9000. GoNotebook training material mounted at runtime via k8s hostPath volume at /app/gonotebook

## Key Functions
- `POST /analyze` -- Main RCA endpoint: receives webhook payload, pre-fetches Datadog data (logs, host info, events, monitor config), invokes Claude for analysis, generates embeddings, stores in Qdrant, creates Datadog Notebook. Uses resolveServiceName() for accurate service identification and deriveSeverity()/deriveEnv() for default values. Registers created notebooks in notebookRegistry for lifecycle tracking
- `POST /recover` -- Notebook lifecycle endpoint: receives recovery webhook, looks up active notebook via notebookRegistry (monitor_id -> notebookId), updates title from [Incident Report] to [RESOLVED], changes Status: ACTIVE to Status: RESOLVED, appends resolution cell with recovery timestamp
- `GET /notebooks/registry` -- Returns the current notebookRegistry map (monitor_id -> {notebookId, monitorName, createdAt, status}) for debugging lifecycle tracking
- `POST /watchdog` -- Watchdog monitor analysis endpoint: similar to /analyze but with watchdog-specific prompt and notebook formatting. Creates "[Watchdog Alert]" titled notebooks with anomaly characterization, impact assessment, and correlation analysis
- `POST /generate-notebook` -- Generates Datadog notebook from analysis results
- `POST /tools/execute` -- Executes dd_lib Python tools
- `POST /tools/create-function` -- Creates new dd_lib tool functions
- `GET /tools` -- Lists available dd_lib tools
- `GET /health` -- Health check
- `GET /go-principles/stats` -- GoNotebook RAG collection stats (point count, sample points, path status)
- `POST /go-principles/reingest` -- Force re-ingestion of goNotebook training material (deletes and recreates collection)
- `POST /github/process-issue` -- Process GitHub issue with Go principles RAG context injection
- `resolveServiceName(payload)` -- Determines actual service name from webhook payload. Priority: APPLICATION_TEAM > scope tag > service (if not monitor type) > fallback. Prevents "http-check" from appearing as service name
- `deriveSeverity(payload)` -- Maps alert_status/priority/URGENCY to industry-standard severity (P2-P5). Ensures notebooks never display 'N/A' for severity
- `deriveEnv(payload)` -- Extracts environment from tags/scope/payload fields, defaults to 'production' instead of 'N/A'
- `invokeClaudeCode(prompt, workDir, context)` -- Invokes Claude Code CLI with dual auth support. Includes automatic OAuth token refresh via refreshOAuthToken() when token is expiring
- `retryWithBackoff(fn, context)` -- Self-healing error recovery with exponential backoff (3 attempts, 1s/2s/4s). Classifies errors via classifyError() and handles token refresh, rate limits (Retry-After), and transient failures
- `refreshOAuthToken()` -- Refreshes OAuth access token using refresh_token from credentials.json. Writes updated tokens back to CLAUDE_CREDS_PATH
- `isTokenExpiringSoon()` -- Checks if OAuth token expires within 5 minutes
- `classifyError(err, stderr)` -- Classifies errors into types: auth_expired, rate_limited, network_error, resource_exhausted, server_error, unknown
- `createDatadogNotebook(monitorId, analysis, data)` -- Creates Datadog API v1 notebook. Registers notebook in notebookRegistry for lifecycle tracking
- `resolveNotebook(notebookId, monitorId, monitorName, recoveryTimestamp)` -- Updates existing notebook to RESOLVED status: changes title, header status, appends resolution cell
- `getDatadogNotebook(notebookId)` -- Fetches notebook from Datadog API v1
- `updateDatadogNotebook(notebookId, notebookData)` -- PUTs updated notebook back to Datadog API v1
- `createFailureEvent(context, err)` -- Creates a Datadog event documenting an agent analysis failure (best-effort alerting)
- `createFailureNotebook(context, err, fullPayload)` -- Creates a Datadog failure notebook with error details, manual investigation steps, and raw payload dump
- `getManualInvestigationSteps(errorType)` -- Returns markdown investigation checklist based on error classification
- `createWatchdogNotebook(payload, analysis, triggerTime, similarRCAs, urls)` -- Creates Watchdog-specific Datadog notebook with "[Watchdog Alert]" title
- `generateEmbeddings(text)` -- Generates vector embeddings via Ollama
- `storeRCA(monitorId, analysis, embedding)` -- Stores RCA in Qdrant
- `searchSimilarRCAs(embedding, limit)` -- Finds similar past incidents
- `httpRequest(url, method, data, extraHeaders)` -- Shared HTTP request helper for Datadog API calls with DD-API-KEY/DD-APPLICATION-KEY headers
- `structuredLog(level, event, data)` -- Structured JSON logging with timestamp, level, event name, and arbitrary data fields
- `initGoPrinciplesCollection()` -- Initializes go_principles Qdrant collection, returns {exists, pointCount}
- `discoverGoNotebookFiles()` -- Walks goNotebook directory, categorizes files by topic
- `chunkMarkdownFile(path, category, topic)` -- Chunks markdown by heading boundaries
- `chunkGoFile(path, category, topic)` -- Wraps Go source file as single chunk with context
- `ingestGoNotebook()` -- Full ingestion pipeline: discover, chunk, embed, upsert to Qdrant
- `searchGoPrinciples(queryText, limit)` -- RAG search with optional category filtering
- `formatGoPrinciplesContext(results)` -- Formats retrieved principles for prompt injection

## Data Types
- Analyze request: `{payload: {monitor_id, monitor_name, alert_status, hostname, ..., APPLICATION_TEAM, URGENCY, ...}}`
- Analyze response: `{success, monitorId, analysis, notebook: {url}, timestamp}` -- on error: `{error, error_type, retries_exhausted, failure_event: {id}, failure_notebook: {id, url}}`
- Recover request: Same as analyze request (payload object with webhook fields, alert_status = "OK" or "Recovered")
- Recover response: `{success, monitorId, monitorName, notebook: {id, url}, status: "resolved", timestamp}`
- Watchdog request: Same as analyze request (payload object with webhook fields)
- Watchdog response: `{success, monitorId, monitorName, monitorType: "watchdog", analysis, triggerTime, similarRCAs, notebook: {id, url}, timestamp}`
- notebookRegistry: `Map<monitor_id, {notebookId, monitorName, createdAt, status}>` -- in-memory lifecycle tracker
- Tools: dd_lib Python functions (get_hosts, get_events, search_logs, etc.)

## Logging
Uses `console.log`, `console.error` with timestamps

## CRUD Entry Points
- **Create**: POST /tools/create-function to add new dd_lib tools
- **Read**: GET /tools lists available tools, POST /analyze runs analysis
- **Update**: Modify SYSTEM_PROMPT, Qdrant collection config, notebook template
- **Delete**: N/A

## Style Guide
- Dual auth: prefers OAuth token (with auto-refresh) over ANTHROPIC_API_KEY
- Pre-fetch pattern: gather Datadog context before invoking Claude
- Self-healing: retryWithBackoff wraps all Claude invocations with exponential backoff and error classification
- Failure alerting: on unrecoverable errors, creates both a Datadog event and a failure notebook for post-mortem
- Notebook lifecycle: notebookRegistry tracks Active -> Resolved transitions per monitor_id
- Service resolution: resolveServiceName() ensures APPLICATION_TEAM is used over monitor-type service fields
- Default values: deriveSeverity() and deriveEnv() prevent 'N/A' from appearing in notebooks
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
