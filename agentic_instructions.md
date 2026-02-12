# agentic_instructions.md -- Rayne Project Root Index

## Purpose
Rayne is a Go-based REST API server that wraps the Datadog API, providing endpoints for downtime, event, host, webhook, RUM visitor tracking, and AI-powered Root Cause Analysis (RCA). It includes a portfolio frontend, a FastAPI chatbot gateway, a Claude AI sidecar for incident analysis, and a Python reference library (`dd_lib`) that also serves as the Claude agent's tool interface.

## Technology
- **Go server** (`mkii_ddog_server/`): Go 1.24, net/http, PostgreSQL, Datadog APM (dd-trace-go)
- **Frontend** (`frontend/`): Go, a-h/templ v0.3.977, HTMX 1.9.10, vanilla JS
- **Gateway** (`gateway/`): Python, FastAPI, WebSocket, httpx, Ollama
- **Claude Agent Sidecar** (`docker/claude-agent/`): Node.js 22, Claude Code CLI, Qdrant, Ollama
- **Python Library** (`dd_lib/`): Python 3, requests
- **Infrastructure**: Docker Compose, Kubernetes/minikube, Helm, Cloudflare Tunnel

## Directory Routing Index

### Go Server -- `mkii_ddog_server/`
Core application. Port of `dd_lib` Python library to Go.

| Directory | Purpose | Doc |
|-----------|---------|-----|
| `mkii_ddog_server/` | Build system, Dockerfile, Go module root | [doc](mkii_ddog_server/agentic_instructions.md) |
| `mkii_ddog_server/cmd/api/` | HTTP server, route registration, CORS, APM middleware | [doc](mkii_ddog_server/cmd/api/agentic_instructions.md) |
| `mkii_ddog_server/cmd/config/` | Environment variable loading (Config struct) | [doc](mkii_ddog_server/cmd/config/agentic_instructions.md) |
| `mkii_ddog_server/cmd/db/` | PostgreSQL connection factory with APM tracing | [doc](mkii_ddog_server/cmd/db/agentic_instructions.md) |
| `mkii_ddog_server/cmd/migrate/` | Application entry point (main.go) | [doc](mkii_ddog_server/cmd/migrate/agentic_instructions.md) |
| `mkii_ddog_server/cmd/types/` | Shared types: User, AlertPayload, AlertEvent | [doc](mkii_ddog_server/cmd/types/agentic_instructions.md) |
| `mkii_ddog_server/cmd/utils/` | HTTP handler helpers: Endpoint(), ParseJson(), WriteJson() | [doc](mkii_ddog_server/cmd/utils/agentic_instructions.md) |
| `mkii_ddog_server/cmd/utils/httpclient/` | Pre-configured HTTP clients with APM tracing | [doc](mkii_ddog_server/cmd/utils/httpclient/agentic_instructions.md) |
| `mkii_ddog_server/cmd/utils/keys/` | Datadog API key retrieval from env vars | [doc](mkii_ddog_server/cmd/utils/keys/agentic_instructions.md) |
| `mkii_ddog_server/cmd/utils/requests/` | Generic HTTP helpers: Get[T], Post[T], Put[T], Delete[T] | [doc](mkii_ddog_server/cmd/utils/requests/agentic_instructions.md) |
| `mkii_ddog_server/cmd/utils/urls/` | Datadog API URL constants and builders | [doc](mkii_ddog_server/cmd/utils/urls/agentic_instructions.md) |
| `mkii_ddog_server/services/webhooks/` | Webhook ingestion, storage, dispatcher, orchestrator | [doc](mkii_ddog_server/services/webhooks/agentic_instructions.md) |
| `mkii_ddog_server/services/webhooks/processors/` | WebhookProcessor implementations (notify, downtime, forward, slack) | [doc](mkii_ddog_server/services/webhooks/processors/agentic_instructions.md) |
| `mkii_ddog_server/services/agents/` | AI agent framework: RLM loop, role classifier, Claude agent | [doc](mkii_ddog_server/services/agents/agentic_instructions.md) |
| `mkii_ddog_server/services/rum/` | RUM visitor tracking: visitors, sessions, events, analytics | [doc](mkii_ddog_server/services/rum/agentic_instructions.md) |
| `mkii_ddog_server/services/accounts/` | Multi-account Datadog credential management | [doc](mkii_ddog_server/services/accounts/agentic_instructions.md) |
| `mkii_ddog_server/services/demo/` | Demo data seeding and intentional error generation | [doc](mkii_ddog_server/services/demo/agentic_instructions.md) |
| `mkii_ddog_server/services/downtimes/` | Datadog downtimes API proxy | [doc](mkii_ddog_server/services/downtimes/agentic_instructions.md) |
| `mkii_ddog_server/services/events/` | Datadog events API proxy | [doc](mkii_ddog_server/services/events/agentic_instructions.md) |
| `mkii_ddog_server/services/hosts/` | Datadog hosts, active hosts, host tags API proxy | [doc](mkii_ddog_server/services/hosts/agentic_instructions.md) |
| `mkii_ddog_server/services/monitors/` | Datadog monitors API proxy with pagination | [doc](mkii_ddog_server/services/monitors/agentic_instructions.md) |
| `mkii_ddog_server/services/logs/` | Datadog logs search API proxy | [doc](mkii_ddog_server/services/logs/agentic_instructions.md) |
| `mkii_ddog_server/services/catalog/` | Datadog Service Catalog API proxy | [doc](mkii_ddog_server/services/catalog/agentic_instructions.md) |
| `mkii_ddog_server/services/pl/` | Private Location container image management (podman/systemd) | [doc](mkii_ddog_server/services/pl/agentic_instructions.md) |
| `mkii_ddog_server/services/user/` | User auth (login, register) and route registration | [doc](mkii_ddog_server/services/user/agentic_instructions.md) |
| `mkii_ddog_server/services/github/` | GitHub webhook ingestion, issue event storage, Claude agent integration | [doc](mkii_ddog_server/services/github/agentic_instructions.md) |

### Frontend -- `frontend/`
Go-based portfolio website serving on port 3000.

| Directory | Purpose | Doc |
|-----------|---------|-----|
| `frontend/` | Go HTTP server, templ rendering, static file serving | [doc](frontend/agentic_instructions.md) |
| `frontend/templates/` | Templ components: layout, index, chat widget | [doc](frontend/templates/agentic_instructions.md) |
| `frontend/static/js/` | Chat WebSocket client, Datadog RUM tracking | [doc](frontend/static/js/agentic_instructions.md) |
| `frontend/static/css/` | Portfolio and chat widget styles | [doc](frontend/static/css/agentic_instructions.md) |

### Gateway -- `gateway/`
FastAPI WebSocket bridge for the Monty chatbot.

| Directory | Purpose | Doc |
|-----------|---------|-----|
| `gateway/` | WebSocket gateway to Ollama LLM with LLMObs tracing | [doc](gateway/agentic_instructions.md) |

### Claude Agent Sidecar -- `docker/claude-agent/`
Node.js server for AI-powered Root Cause Analysis.

| Directory | Purpose | Doc |
|-----------|---------|-----|
| `docker/claude-agent/` | Claude Code CLI wrapper, Qdrant vector search, notebook generation | [doc](docker/claude-agent/agentic_instructions.md) |

### Authentication & Credentials

| Component | Auth Method | Details |
|-----------|-------------|---------|
| **Claude Code CLI** | OAuth long-term tokens (subscription) | Uses `~/.claude/.credentials.json` with `claudeAiOauth` refresh tokens. **Do NOT use `ANTHROPIC_API_KEY`**. Tokens are provisioned via `claude login` on the host and injected via k8s secret `claude-credentials`. |
| **GitHub CLI (`gh`)** | Personal Access Token | `GH_TOKEN` env var. Required for posting comments back to GitHub issues. |
| **Datadog API** | API + App keys | `DD_API_KEY`, `DD_APP_KEY` env vars. Target: `https://api.ddog-gov.com` |
| **GitHub Webhooks** | HMAC-SHA256 | `GITHUB_WEBHOOK_SECRET` env var. Verified on every incoming webhook. |

### Python Library -- `dd_lib/`
Reference implementation and Claude agent tool interface.

| Directory | Purpose | Doc |
|-----------|---------|-----|
| `dd_lib/` | Top-level scripts and subdirectory index | [doc](dd_lib/agentic_instructions.md) |
| `dd_lib/dd_lib/` | Core library: keys, headers, tools wrapper, API modules | [doc](dd_lib/dd_lib/agentic_instructions.md) |
| `dd_lib/services/` | Service-specific implementations by feature area | [doc](dd_lib/services/agentic_instructions.md) |
| `dd_lib/services/apm/` | APM billing, service listing, metadata creation | [doc](dd_lib/services/apm/agentic_instructions.md) |
| `dd_lib/services/downtimes/` | Downtime creation, monitor listing, testing | [doc](dd_lib/services/downtimes/agentic_instructions.md) |
| `dd_lib/services/monitors/` | Monitor listing with pagination | [doc](dd_lib/services/monitors/agentic_instructions.md) |
| `dd_lib/services/webhooks/` | Webhook creation and event retrieval | [doc](dd_lib/services/webhooks/agentic_instructions.md) |
| `dd_lib/services/rum/` | RUM application creation and event retrieval | [doc](dd_lib/services/rum/agentic_instructions.md) |
| `dd_lib/services/aws_integrations_check/` | AWS integration status checking | [doc](dd_lib/services/aws_integrations_check/agentic_instructions.md) |
| `dd_lib/user_mods/` | User and role management scripts | [doc](dd_lib/user_mods/agentic_instructions.md) |
| `dd_lib/fanout/` | Fan-out utility for parallel API calls | [doc](dd_lib/fanout/agentic_instructions.md) |
| `dd_lib/forecasts/` | Forecast scripts (1-week, annual) | [doc](dd_lib/forecasts/agentic_instructions.md) |
| `dd_lib/http_checks/` | HTTP check utilities | [doc](dd_lib/http_checks/agentic_instructions.md) |
| `dd_lib/implementations/` | Implementation scripts with intake sub-module | [doc](dd_lib/implementations/agentic_instructions.md) |
| `dd_lib/integrations/` | Datadog integration management | [doc](dd_lib/integrations/agentic_instructions.md) |
| `dd_lib/monitors/` | Monitor management scripts | [doc](dd_lib/monitors/agentic_instructions.md) |
| `dd_lib/go_dd_lib/` | Experimental Go port (superseded by mkii_ddog_server) | [doc](dd_lib/go_dd_lib/agentic_instructions.md) |
| `dd_lib/private_locations_script_generator/` | Synthetics private location setup | [doc](dd_lib/private_locations_script_generator/agentic_instructions.md) |

### Infrastructure & Operations

| Directory | Purpose | Doc |
|-----------|---------|-----|
| `k8s/` | Kubernetes manifests for minikube deployment | [doc](k8s/agentic_instructions.md) |
| `helm/` | Helm chart values | [doc](helm/agentic_instructions.md) |
| `scripts/` | Dev/deployment scripts (minikube setup, traffic gen, DNS) | [doc](scripts/agentic_instructions.md) |
| `assets/` | Incident report templates for Claude agent | [doc](assets/agentic_instructions.md) |
| `docs/` | Feature documentation | [doc](docs/agentic_instructions.md) |
| `prompts/` | AI prompt templates and TODO tracking | [doc](prompts/agentic_instructions.md) |
| `troubleshooting/` | Troubleshooting guides for deployment/runtime issues | [doc](troubleshooting/agentic_instructions.md) |

---

## Key Abstractions

### Webhook Processing

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **WebhookProcessor** | interface | `mkii_ddog_server/services/webhooks/types.go` | Implemented by all processors in `processors/`. Consumed by `ProcessorOrchestrator`. | **Create**: add new file in `processors/`, implement `Name()`, `CanProcess()`, `Process()`. **Read**: `processors/` dir. **Update**: modify `CanProcess()` filtering. **Delete**: remove file, unregister from orchestrator. |
| **WebhookPayload** | struct | `mkii_ddog_server/services/webhooks/types.go` | Ingested by `Handler.ReceiveWebhook()`, stored in `WebhookEvent`, passed to all processors. Maps to `AlertPayload` in `cmd/types/alert.go`. | **Create**: N/A (received from Datadog). **Read**: `types.go`. **Update**: add fields to struct and JSON tags. **Delete**: N/A. |
| **Dispatcher** | struct | `mkii_ddog_server/services/webhooks/dispatcher.go` | Receives events from `Handler`, routes to `ProcessorOrchestrator`. Initialized in `cmd/api/api.go`. | **Create**: instantiated in `api.go`. **Read**: `dispatcher.go`. **Update**: adjust worker count, queue capacity. **Delete**: N/A. |
| **ProcessorOrchestrator** | struct | `mkii_ddog_server/services/webhooks/orchestrator.go` | Runs tiered processing: registers `WebhookProcessor` impls (Tier 1: fast parallel) and `AgentOrchestrator` (Tier 2: RCA). | **Create**: instantiated in `api.go`. **Read**: `orchestrator.go`. **Update**: register/unregister processors, change tier assignments. **Delete**: N/A. |

### GitHub Issue Processing

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Handler** | struct | `mkii_ddog_server/services/github/handler.go` | Receives GitHub issue webhooks, verifies HMAC, deduplicates by delivery_id, stores events, triggers agent processing for "opened" issues. | **Create**: POST `/v1/webhooks/github/issues`. **Read**: GET `/v1/webhooks/github/issues`, `/{id}`, `/stats`. |
| **AgentClient** | struct | `mkii_ddog_server/services/github/agent_client.go` | HTTP client calling Claude agent sidecar at `CLAUDE_AGENT_URL/github/process-issue`. 15-min timeout. | **Create**: instantiated in `NewHandler()`. **Read**: `agent_client.go`. |
| **Notifier** | struct | `mkii_ddog_server/services/github/notifier.go` | Sends desktop notifications via notify-server. Gray border (#808080). Fire-and-forget goroutine. | **Create**: instantiated in `NewHandler()`. **Read**: `notifier.go`. |
| **Storage** | struct | `mkii_ddog_server/services/github/storage.go` | PostgreSQL `github_issue_events` table with delivery_id dedup, agent processing status tracking. | **Create**: `StoreEvent()`. **Read**: `GetRecentEvents()`, `GetEventByID()`, `GetStats()`. **Update**: `UpdateAgentStatus()`. |

### Agent Analysis (RCA)

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Agent** | interface | `mkii_ddog_server/services/agents/types.go` | Implemented by `ClaudeAgent`. Orchestrated by `AgentOrchestrator` via `RLMCoordinator`. | **Create**: implement `Agent` interface, register via `orchestrator.RegisterAgent()`. **Read**: `types.go`. **Update**: modify role classification rules. **Delete**: unregister agent. |
| **AgentOrchestrator** | struct | `mkii_ddog_server/services/agents/orchestrator.go` | Invoked by `ProcessorOrchestrator` (Tier 2). Contains `RoleClassifier` and `RLMCoordinator`. Bounded concurrency via semaphore. | **Create**: instantiated in `api.go`. **Read**: `orchestrator.go`. **Update**: adjust `MaxConcurrent`, `RLMMaxIterations`. **Delete**: N/A. |
| **RoleClassifier** | struct | `mkii_ddog_server/services/agents/classifier.go` | Used by `AgentOrchestrator` to route alerts to specialist agents by role (Infrastructure, Application, Database, Network, Logs, General). | **Create**: N/A. **Read**: `classifier.go`. **Update**: add rules to `monitorTypeRules`, `tagRules`, `servicePatterns`, `hostnamePatterns`. **Delete**: N/A. |
| **RLMCoordinator** | struct | `mkii_ddog_server/services/agents/rlm.go` | Implements Plan->Query->Analyze->Conclude loop. Coordinates `Agent` and `SubAgent` instances. | **Create**: N/A. **Read**: `rlm.go`. **Update**: adjust `maxIterations`. **Delete**: N/A. |
| **ClaudeAgent** | struct | `mkii_ddog_server/services/agents/claude_agent.go` | Implements `Agent` interface. Calls Claude AI sidecar at `CLAUDE_AGENT_URL/analyze`. | **Create**: N/A. **Read**: `claude_agent.go`. **Update**: modify sidecar URL or request payload. **Delete**: N/A. |

### Claude Agent Sidecar

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Agent Server** | service | `docker/claude-agent/agent-server.js` | Invoked by `ClaudeAgent` in Go server. Uses `dd_lib` Python tools. Stores results in Qdrant. Creates Datadog Notebooks. | **Create**: N/A. **Read**: `agent-server.js`. **Update**: modify endpoints, system prompt, notebook template. **Delete**: N/A. |
| **TOOLS Registry** | dict | `dd_lib/dd_lib/dd_lib_tools.py` | Maps tool names to Python functions. Called by agent sidecar via `/tools/execute`. | **Create**: add function, register in TOOLS dict. **Read**: `dd_lib_tools.py`. **Update**: modify tool implementations. **Delete**: remove from TOOLS dict. |

### RUM Visitor Tracking

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Visitor** | struct | `mkii_ddog_server/services/rum/types.go` | Created by `InitVisitor` handler. Has many `Session`s. Tracked by frontend `datadog-rum-init.js`. | **Create**: POST `/v1/rum/init`. **Read**: `types.go`, `storage.go`. **Update**: `UpdateVisitorLastSeen()`. **Delete**: cascade via PostgreSQL FK. |
| **Session** | struct | `mkii_ddog_server/services/rum/types.go` | Belongs to `Visitor`. Has many `RUMEvent`s. Created on each `InitVisitor` call. | **Create**: POST `/v1/rum/init`. **Read**: GET `/v1/rum/sessions`. **Update**: POST `/v1/rum/session/end`. **Delete**: cascade via FK. |
| **RUMEvent** | struct | `mkii_ddog_server/services/rum/types.go` | Belongs to `Session`. Types: view, action, error, resource, long_task. | **Create**: POST `/v1/rum/track`. **Read**: via analytics. **Update**: N/A. **Delete**: cascade via FK. |

### Multi-Account Credentials

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Account** | struct | `mkii_ddog_server/services/accounts/types.go` | Converted to `Credentials` via `ToCredentials()`. Used by `DowntimeProcessor`, `Handler` for multi-org Datadog API calls. | **Create**: `CreateAccountRequest`. **Read**: `types.go`. **Update**: `UpdateAccountRequest` (pointer fields for partial updates). **Delete**: N/A (storage not yet in this package). |
| **Credentials** | struct | `mkii_ddog_server/services/accounts/types.go` | Used by `requests.GetWithCreds[T]()`, `PostWithCreds[T]()` etc. Contains APIKey, AppKey, BaseURL. | **Create**: via `Account.ToCredentials()`. **Read**: `types.go`. **Update**: N/A. **Delete**: N/A. |
| **AccountResolver** | interface | `mkii_ddog_server/services/webhooks/types.go` | Implemented by account storage. Used by `Handler` to resolve credentials per webhook org. | **Create**: implement interface. **Read**: `types.go`. **Update**: N/A. **Delete**: N/A. |

### Datadog API Proxies

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Downtimes** | handlers | `mkii_ddog_server/services/downtimes/` | Uses `requests.Get[T]`, `urls.GetDowntimesUrl` | **Read**: GET `/v1/downtimes`. |
| **Events** | handlers | `mkii_ddog_server/services/events/` | Uses `requests.Get[T]`, `urls.GetEvents` | **Read**: GET `/v1/events`. |
| **Hosts** | handlers | `mkii_ddog_server/services/hosts/` | Uses `requests.Get[T]`, `urls.GetHosts`, `GetHostTags()` | **Read**: GET `/v1/hosts`, `/v1/hosts/active`, `/v1/hosts/{hostname}/tags`. |
| **Monitors** | handlers | `mkii_ddog_server/services/monitors/` | Uses `requests.Get[T]`, `urls.SearchMontiors` | **Read**: GET `/v1/monitors`, `/v1/monitors/triggered`, `/v1/monitors/{id}`. |
| **Logs** | handlers | `mkii_ddog_server/services/logs/` | Uses `requests.Post[T]`, `urls.LogSearch` | **Read**: POST `/v1/logs/search`, `/v1/logs/search/advanced`. |
| **Service Catalog** | handlers | `mkii_ddog_server/services/catalog/` | Uses `requests.Get[T]`/`Post[T]`, `urls.ServiceDefinitions` | **Create**: POST catalog endpoint. **Read**: GET `/v1/catalog/services`. |

### HTTP Infrastructure

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **DDogServer** | struct | `mkii_ddog_server/cmd/api/api.go` | Wires all storages, handlers, dispatchers. Registers all routes. | **Create**: instantiated in `main.go`. **Read**: `api.go`. **Update**: add routes in `Run()`. **Delete**: N/A. |
| **Endpoint/EndpointWithPathParams** | functions | `mkii_ddog_server/cmd/utils/utils.go` | Used by all route registrations. Wraps handler `(int, any)` pattern. | **Read**: `utils.go`. **Update**: modify response encoding or error logging. |
| **requests.Get[T]/Post[T]/...** | generic funcs | `mkii_ddog_server/cmd/utils/requests/requests.go` | Used by all Datadog proxy services. Adds auth headers, APM tracing. | **Read**: `requests.go`. **Update**: modify header injection or tracing. |
| **HTTP Clients** | vars | `mkii_ddog_server/cmd/utils/httpclient/client.go` | DefaultClient, AgentClient, NotifyClient, ForwardingClient, DatadogClient. All APM-traced. | **Read**: `client.go`. **Update**: adjust timeouts or pool sizes. |
| **Config** | struct | `mkii_ddog_server/cmd/config/env.go` | Read by `main.go`, `db.go`, tracer. Loaded once as package-level `Envs`. | **Read**: `env.go`. **Update**: add fields and env var mappings. |

### Frontend & Gateway

| Abstraction | Type | Location | Relationships | CRUD Routing |
|-------------|------|----------|---------------|--------------|
| **Templ Components** | templ | `frontend/templates/*.templ` | Layout wraps Index, Index includes ChatWidget. Rendered by `main.go`. | **Create**: add `.templ` file, run `templ generate`. **Read**: `templates/`. **Update**: edit `.templ`, regenerate, commit `_templ.go`. |
| **Chat WebSocket Client** | JS | `frontend/static/js/chat.js` | Connects to Gateway WebSocket. Protocol-aware (ws/wss). | **Read**: `chat.js`. **Update**: modify reconnect logic or message handling. |
| **RUM Tracker** | JS | `frontend/static/js/datadog-rum-init.js` | Calls Rayne server `/v1/rum/init`, `/v1/rum/track`. Stores UUID in localStorage. | **Read**: `datadog-rum-init.js`. **Update**: modify tracking events or API calls. |
| **Gateway** | FastAPI app | `gateway/main.py` | Receives WebSocket from frontend, streams to Ollama, returns tokens. LLMObs tracing. | **Read**: `main.py`. **Update**: modify `SYSTEM_PROMPT`, `OLLAMA_MODEL`, or streaming logic. |

---

## Excluded Directories

| Directory | Reason |
|-----------|--------|
| `*/__pycache__/` | Python bytecode cache |
| `*/.claude/` | IDE/agent tool configuration |
| `.git/` | Version control system |
| `*/bin/` | Build output artifacts |
| `*/vendor/` | Vendored Go dependencies |
| `*/node_modules/` | Node.js dependencies |
| `*/.venv/`, `*/venv/` | Python virtual environments |
| `dd_lib/services/*/dd/` | Copied utility sub-packages (documented in parent dirs) |
| `dd_lib/services/apm/nrm/` | Data files only (xlsx, json, tf) -- no parseable source code |
| `dd_lib/services/tags/` | Contains only `dd/` utility sub-package, no scripts |
| `dd_lib/services/logs/` | Empty directory |
| `dd_lib/roles/` | Empty directory |
| `mkii_ddog_server/services/privateLocationImageUpdate/` | Empty directory |
| `docker/`, `frontend/static/`, `mkii_ddog_server/cmd/`, `mkii_ddog_server/services/` | Structural containers -- all children documented individually |
