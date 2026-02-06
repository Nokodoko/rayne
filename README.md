# Rayne

A Go-based REST API server that wraps the Datadog API, providing endpoints to manage downtimes, events, hosts, webhooks, and RUM (Real User Monitoring) visitor tracking. Features AI-powered Root Cause Analysis using Claude Code with vector database storage for learning from past incidents.

## Features

- **Multi-Account Support**: Manage multiple Datadog organizations (US Gov, Commercial, EU, etc.) from a single instance
- **Datadog API Proxy**: Downtimes, events, hosts, and private location management
- **Webhook Handling**: Receive, store, and process Datadog webhooks with auto-downtime triggers and automatic account routing
- **AI-Powered RCA**: Automatic Root Cause Analysis via Claude Code when alerts trigger
- **Incident Report Notebooks**: Auto-generated Datadog Notebooks with hyperlinks to all referenced resources
- **Pre-fetched Datadog Context**: Logs, events, host info, and monitor details fetched before RCA analysis
- **Vector DB Storage**: Store and retrieve past RCAs using Qdrant for pattern matching
- **Local Embeddings**: Generate embeddings with Ollama (Gemma 2B) for similarity search
- **dd_lib Python Tools**: Extensible Python tools for Datadog API with auto-write capability
- **Desktop Notifications**: Local notification server with Dunst integration (orange border alerts)
- **RUM Visitor Tracking**: Server-generated UUIDs, session management, and analytics
- **Demo Data Generators**: Seed fake data for demonstrations
- **Kubernetes Ready**: Includes manifests for minikube deployment

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Rayne Pod                                │
│  ┌──────────────────┐       ┌─────────────────────────────────┐ │
│  │  rayne (Go API)  │ HTTP  │  claude-agent sidecar           │ │
│  │  Port 8080       │◄─────►│  Port 9000                      │ │
│  │                  │       │  - Claude Code CLI              │ │
│  │  Webhook →       │       │  - dd_lib Python tools          │ │
│  │  Alert/Warn      │       │  - Ollama embeddings            │ │
│  │  triggers        │       │  - Qdrant vector storage        │ │
│  │  /analyze        │       │  - assets templates             │ │
│  └──────────────────┘       └─────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
         │                           │                    │
         ▼                           ▼                    ▼
   ┌──────────┐              ┌──────────────┐      ┌──────────────┐
   │ Postgres │              │    Qdrant    │      │   Ollama     │
   │  :5432   │              │ (RCA storage)│      │ (embeddings) │
   └──────────┘              └──────────────┘      └──────────────┘
```

## Prerequisites

- Go 1.22+ (for local development)
- Docker & Docker Compose (recommended)
- PostgreSQL 16+ (if running locally without Docker)
- Datadog API and Application keys
- Anthropic API key (for Claude Code RCA analysis)

## Quick Start

### Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/Nokodoko/rayne.git
cd rayne

# Set your API keys
export DD_API_KEY=your-datadog-api-key
export DD_APP_KEY=your-datadog-app-key

# Build and start
docker-compose up -d --build

# Check health
curl localhost:8080/health

# Seed demo data
curl -X POST localhost:8080/v1/demo/seed/all

# View logs
docker-compose logs -f rayne

# Stop
docker-compose down
```

### Local Development

```bash
cd mkii_ddog_server

# Set environment variables
export DD_API_KEY=your-api-key
export DD_APP_KEY=your-app-key
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=rayne
export DB_PASSWORD=raynepassword
export DB_NAME=rayne

# Ensure PostgreSQL is running with the above credentials

# Run directly
make r

# Or build and run
make build
./bin/rayne
```

### Minikube Deployment (Full Stack)

The minikube deployment includes:
- Rayne API server with Claude Agent sidecar
- PostgreSQL for webhook/RUM data
- Qdrant vector database for RCA storage
- Ollama with Gemma 2B for embeddings
- Datadog Agent for APM

```bash
# Set your API keys
export TF_VAR_ecco_dd_api_key=your-datadog-api-key
export TF_VAR_ecco_dd_app_key=your-datadog-app-key
export ANTHROPIC_API_KEY=your-anthropic-api-key  # Required for RCA

# Run the setup script (requires ~12GB RAM for minikube)
./scripts/minikube-setup.sh

# Get service URL
minikube service rayne-service --url

# Test webhook with RCA trigger (creates Datadog Notebook with hyperlinks)
curl -X POST $(minikube service rayne-service --url)/v1/webhooks/receive \
  -H "Content-Type: application/json" \
  -d '{
    "monitor_id": 12345678,
    "monitor_name": "High CPU Alert",
    "alert_status": "Alert",
    "hostname": "web-server-01.prod.example.com",
    "service": "api-gateway",
    "scope": "host:web-server-01",
    "APPLICATION_TEAM": "platform-engineering",
    "tags": ["env:production", "team:platform"]
  }'

# Response includes notebook URL:
# {"event_id":1,"notebook":{"id":"13768151","url":"https://app.datadoghq.com/notebook/13768151"}}
```

### Helm Deployment (Datadog Agent)

The Helm chart supports multiple ways to provide Datadog API keys:

**Option 1: Environment variable interpolation with envsubst**
```bash
export TF_VAR_ecco_dd_api_key=your-api-key
export TF_VAR_ecco_dd_app_key=your-app-key

# Substitute variables and install
envsubst < helm/values.yaml | helm install datadog-agent datadog/datadog -f -
```

**Option 2: Using --set flags**
```bash
helm install datadog-agent datadog/datadog \
  --set datadog.apiKey=$TF_VAR_ecco_dd_api_key \
  --set datadog.appKey=$TF_VAR_ecco_dd_app_key \
  -f helm/values.yaml
```

**Option 3: Using existing Kubernetes secrets**
```bash
# Create secrets first
kubectl create secret generic datadog-secrets \
  --from-literal=api-key="$TF_VAR_ecco_dd_api_key" \
  --from-literal=app-key="$TF_VAR_ecco_dd_app_key"

# Install with secret references
helm install datadog-agent datadog/datadog \
  --set datadog.apiKeyExistingSecret=datadog-secrets \
  --set datadog.appKeyExistingSecret=datadog-secrets \
  -f helm/values.yaml
```

## API Endpoints

### Health & Auth
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| POST | `/login` | User login |
| POST | `/register` | User registration |

### Datadog Proxies
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/downtimes` | List Datadog downtimes |
| GET | `/v1/events` | List Datadog events |
| GET | `/v1/hosts` | List hosts |
| GET | `/v1/hosts/active` | Get active host count |
| GET | `/v1/hosts/{hostname}/tags` | Get tags for a host |
| GET | `/v1/pl/refresh/{action}` | Private location container ops |

### Webhooks
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/webhooks/receive` | Receive Datadog webhooks (triggers RCA on Alert/Warn) |
| POST | `/v1/webhooks/receive/{account}` | Receive webhook with explicit account routing |
| GET | `/v1/webhooks/events` | List stored webhook events |
| POST | `/v1/webhooks/create` | Create webhook in Datadog |
| GET | `/v1/webhooks/stats` | Webhook statistics |
| POST | `/v1/webhooks/config` | Save webhook configuration |

### Multi-Account Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/accounts` | List all Datadog accounts (credentials hidden) |
| POST | `/v1/accounts` | Create new Datadog account |
| GET | `/v1/accounts/{name}` | Get account by name |
| PUT | `/v1/accounts/{name}` | Update account |
| DELETE | `/v1/accounts/{name}` | Delete account |
| POST | `/v1/accounts/{name}/default` | Set account as default |
| POST | `/v1/accounts/{name}/test` | Test account credentials against Datadog API |

## Webhook Payload

When sending webhooks to `/v1/webhooks/receive`, the following JSON payload format is expected:

### Standard Datadog Fields

| Field | Type | Description |
|-------|------|-------------|
| `alert_id` | int64 | Unique alert identifier |
| `alert_title` | string | Title of the alert |
| `alert_message` | string | Alert message body |
| `alert_status` | string | Alert state: `"Alert"`, `"OK"`, `"Warn"`, or `"No Data"` |
| `monitor_id` | int64 | Datadog monitor ID |
| `monitor_name` | string | Name of the monitor |
| `monitor_type` | string | Type of monitor (e.g., `"metric alert"`, `"service check"`) |
| `tags` | string[] | Array of tags (e.g., `["env:production", "team:platform"]`) |
| `timestamp` | int64 | Unix timestamp of the event |
| `event_type` | string | Type of event |
| `priority` | string | Alert priority |
| `hostname` | string | Affected hostname |
| `service` | string | Affected service name |
| `scope` | string | Alert scope (e.g., `"host:web-server-01"`) |
| `transition_id` | string | Unique transition identifier |
| `last_updated` | int64 | Last update timestamp |
| `snapshot_url` | string | URL to alert snapshot |
| `link` | string | Link to the monitor in Datadog |
| `org_id` | int64 | Datadog organization ID (used for multi-account routing) |
| `org_name` | string | Organization name |

### Custom Fields (Terraform Webhook Config)

| Field | Type | Description |
|-------|------|-------------|
| `ALERT_STATE` | string | Custom alert state |
| `ALERT_TITLE` | string | Custom alert title |
| `APPLICATION_LONGNAME` | string | Full application name |
| `APPLICATION_TEAM` | string | Responsible team |
| `DETAILED_DESCRIPTION` | string | Detailed alert description |
| `IMPACT` | string | Impact assessment |
| `METRIC` | string | Affected metric |
| `SUPPORT_GROUP` | string | Support group |
| `THRESHOLD` | string | Alert threshold value |
| `VALUE` | string | Current metric value |
| `URGENCY` | string | Alert urgency level |

### Example Payload

```json
{
  "alert_id": 123456789,
  "alert_title": "High CPU Usage on web-server-01",
  "alert_message": "CPU usage exceeded 90% threshold",
  "alert_status": "Alert",
  "monitor_id": 12345678,
  "monitor_name": "High CPU Alert",
  "monitor_type": "metric alert",
  "tags": ["env:production", "team:platform", "service:api-gateway"],
  "timestamp": 1706540400,
  "event_type": "alert",
  "priority": "normal",
  "hostname": "web-server-01.prod.example.com",
  "service": "api-gateway",
  "scope": "host:web-server-01",
  "transition_id": "abc123",
  "last_updated": 1706540400,
  "snapshot_url": "https://app.datadoghq.com/snapshot/...",
  "link": "https://app.datadoghq.com/monitors/12345678",
  "org_id": 123456,
  "org_name": "My Organization",
  "APPLICATION_TEAM": "platform-engineering",
  "SUPPORT_GROUP": "sre-oncall",
  "URGENCY": "high"
}
```

### Multi-Account Routing

Webhooks are routed to the appropriate Datadog account using:

1. **Explicit routing** via path parameter: `POST /v1/webhooks/receive/{account_name}`
2. **Automatic routing** via `org_id` field in the payload (matched against stored accounts)
3. **Default account** fallback when no match is found

This allows a single Rayne instance to manage webhooks from multiple Datadog organizations (US Gov, Commercial, EU, etc.).

### RUM Visitor Tracking
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/rum/init` | Initialize visitor (generates UUID) |
| POST | `/v1/rum/track` | Track RUM events |
| POST | `/v1/rum/session/end` | End a session |
| GET | `/v1/rum/visitors` | Get unique visitor count |
| GET | `/v1/rum/analytics` | Get visitor analytics |
| GET | `/v1/rum/sessions` | List recent sessions |

## Frontend RUM Integration

The frontend includes Datadog Real User Monitoring (RUM) integration that links browser sessions to backend-generated UUIDs stored in PostgreSQL.

### How It Works

1. **SDK Load**: The Datadog Browser RUM SDK is loaded from CDN on page load
2. **Backend Init**: `POST /v1/rum/init` is called to get/create a visitor UUID
3. **User Identity**: `DD_RUM.setUser({ id: visitor_uuid })` links Datadog sessions to our UUID
4. **Event Tracking**: Page views and custom events are tracked to both Datadog and the backend
5. **Session End**: `beforeunload` and `visibilitychange` events trigger session end tracking

### Architecture Flow

```
Browser                          Backend (8080)         Datadog
   │                                  │                    │
   │ 1. Load DD_RUM SDK               │                    │
   │ 2. POST /v1/rum/init ──────────► │                    │
   │ ◄─────── visitor_uuid, session_id│                    │
   │ 3. DD_RUM.setUser({id: uuid})    │                    │
   │ 4. DD_RUM sends events ──────────┼──────────────────► │
   │    (with @usr.id = our uuid)     │                    │
   │ 5. POST /v1/rum/track ──────────►│                    │
   │                                  │ PostgreSQL         │
```

### Configuration

The frontend RUM script uses `window.RAYNE_API_BASE` to determine the backend URL:

```html
<script>
    window.RAYNE_API_BASE = 'http://localhost:8080';  <!-- Set to your backend URL -->
</script>
<script src="/static/js/datadog-rum-init.js"></script>
```

### localStorage Keys

| Key | Description |
|-----|-------------|
| `rayne_visitor_uuid` | Persistent visitor UUID from backend |
| `rayne_session_id` | Current session ID |
| `rayne_session_start` | Session start timestamp (ms) |

### Viewing in Datadog Console

1. Navigate to **RUM** → **Explorer**
2. Filter by `service:rayne-frontend`
3. Group by `@usr.id` to see unique visitors by backend UUID
4. Click a session to see full user journey with our UUID as the user identifier

### Custom Event Tracking

Use the global `window.rayneRUM` object for custom event tracking:

```javascript
// Track a custom event (sends to both Datadog and backend)
window.rayneRUM.trackEvent('button_click', {
    button_id: 'submit-form',
    form_name: 'contact'
});

// Access current visitor info
console.log(window.rayneRUM.visitorUuid);
console.log(window.rayneRUM.sessionId);
```

### Demo Data
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/demo/seed/all` | Seed all demo data |
| POST | `/v1/demo/seed/webhooks?count=50` | Seed webhook events |
| POST | `/v1/demo/seed/rum?count=100` | Seed RUM sessions |
| GET | `/v1/demo/monitors` | Generate sample monitors |
| GET | `/v1/demo/status` | Get demo environment status |

### Claude Agent (Internal Sidecar - port 9000)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Claude Agent health (includes dd_lib/assets status) |
| POST | `/analyze` | Trigger RCA analysis with full payload, pre-fetched data, and notebook creation |
| POST | `/generate-notebook` | Generate Datadog notebook from analysis |
| GET | `/templates` | List available incident report templates |
| GET | `/tools` | List available dd_lib tools |
| POST | `/tools/execute` | Execute a dd_lib tool (e.g., search_logs, get_host_info) |
| POST | `/tools/create-function` | Create new dd_lib function (auto-write mode) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DD_API_KEY` | Yes* | - | Datadog API key (*creates default account on startup if no accounts exist) |
| `DD_APP_KEY` | Yes* | - | Datadog Application key (*creates default account on startup if no accounts exist) |
| `DD_API_URL` | No | https://api.ddog-gov.com | Default Datadog API URL |

### Supported Datadog Regions

When creating accounts via `/v1/accounts`, use these base URLs:

| Region | Base URL |
|--------|----------|
| US Government | `https://api.ddog-gov.com` |
| US Commercial | `https://api.datadoghq.com` |
| EU | `https://api.datadoghq.eu` |
| US3 | `https://api.us3.datadoghq.com` |
| US5 | `https://api.us5.datadoghq.com` |
| AP1 (Asia-Pacific) | `https://api.ap1.datadoghq.com` |
| `ANTHROPIC_API_KEY` | No* | - | Anthropic API key (*not needed if using Claude OAuth) |
| `DB_HOST` | No | localhost | PostgreSQL host |
| `DB_PORT` | No | 5432 | PostgreSQL port |
| `DB_USER` | No | - | PostgreSQL user |
| `DB_PASSWORD` | No | - | PostgreSQL password |
| `DB_NAME` | No | - | PostgreSQL database |
| `CLAUDE_AGENT_URL` | No | http://localhost:9000 | Claude Agent sidecar URL |
| `QDRANT_URL` | No | http://qdrant-service:6333 | Qdrant vector DB URL |
| `OLLAMA_URL` | No | http://ollama-service:11434 | Ollama embeddings URL |

## Project Structure

```
rayne/
├── mkii_ddog_server/     # Go server
│   ├── cmd/              # Entry point and utilities
│   ├── services/         # Service handlers
│   │   ├── accounts/     # Multi-account Datadog management
│   │   ├── demo/         # Demo data generators
│   │   ├── downtimes/    # Datadog downtimes
│   │   ├── events/       # Datadog events
│   │   ├── hosts/        # Datadog hosts
│   │   ├── pl/           # Private locations
│   │   ├── rum/          # RUM tracking
│   │   ├── user/         # User management
│   │   └── webhooks/     # Webhook handling + Claude invocation
│   │       └── processors/   # Webhook processors (claude_agent.go)
│   └── Dockerfile
├── docker/
│   └── claude-agent/     # Claude Agent sidecar
│       ├── Dockerfile    # Node.js + Python + Claude Code CLI
│       └── agent-server.js # HTTP wrapper for Claude Code (RCA, notebooks, tools)
├── dd_lib/               # Python Datadog tools (baked into sidecar)
│   └── dd_lib/
│       ├── dd_lib_tools.py   # CLI wrapper for tools (search_logs, get_host_info, etc.)
│       ├── headers.py        # Datadog API headers
│       └── keys.py           # API key management
├── assets/               # Incident report templates
│   ├── incident_report.json      # JSON template for structured reports
│   ├── incident-report-cloned.md # Markdown incident report template
│   └── logs-analysis.md          # Log analysis template
├── k8s/                  # Kubernetes manifests
│   ├── rayne-deployment.yaml      # Rayne + Claude Agent sidecar (with dd_lib volume)
│   ├── postgres-deployment.yaml   # PostgreSQL
│   ├── qdrant-deployment.yaml     # Qdrant vector DB
│   ├── ollama-deployment.yaml     # Ollama (Gemma 2B)
│   ├── assets-configmap.yaml      # Incident templates
│   └── anthropic-secrets.yaml     # Anthropic API key
├── helm/                 # Helm values for Datadog Agent
├── scripts/
│   ├── minikube-setup.sh         # Full minikube deployment script
│   ├── notify-server.py          # Desktop notification server (Dunst)
│   └── traffic-generator.sh      # API traffic generator with failure injection
└── docker-compose.yaml
```

## AI-Powered Root Cause Analysis

When a webhook arrives with `alert_status: "Alert"` or `alert_status: "Warn"`:

1. **Rayne** receives the webhook and stores it in PostgreSQL
2. **Claude Agent** is invoked with the full webhook payload
3. **Pre-fetch Datadog Data**: Before analysis, the agent fetches:
   - Recent error/warning logs (filtered by host or service)
   - Host information and metrics
   - Recent events from the last 30 minutes
   - Monitor configuration details
4. **Ollama** generates embeddings for the alert to find similar past incidents
5. **Qdrant** searches for similar RCAs from previous incidents
6. **Claude Code** analyzes the alert with evidence-based context from:
   - Live Datadog logs, events, and host metrics
   - Past similar RCAs
   - Python dd_lib tools for additional Datadog API access
   - Incident report templates from assets/
7. **Datadog Notebook** is created automatically with the incident report
8. The analysis is stored back in Qdrant for future reference

### Incident Report Notebooks

Each RCA automatically creates a Datadog Notebook containing:

- **Header Section**: Alert summary with hyperlinked monitor ID, hostname, and service
- **Quick Links**: Direct links to all relevant Datadog resources:
  - Monitor (view/edit)
  - Logs (filtered by host, service, or errors)
  - APM Service overview
  - APM Traces (all and error-only)
  - Host Infrastructure dashboard
  - Metrics Explorer
  - Events Explorer
  - Database Monitoring (if applicable)
- **Root Cause Analysis**: Claude's evidence-based analysis
- **CPU Metrics Graph**: Timeseries widget for host CPU usage
- **Related Logs Section**: Links to filtered log views
- **APM & Traces Section**: Links to service traces and errors
- **Similar Past Incidents**: Previous RCAs with similarity scores (hyperlinked)
- **Footer Actions**: Quick links to view/edit monitor and related events

All hyperlinks include time-range filters (last 30 minutes) and relevant query parameters.

### Templates Available

- `incident_report.json` - JSON template for structured incident reports
- `incident-report-cloned.md` - Full incident report template with sections for Summary, Outage Tracking, Response Breakdown, and Organizational Analysis
- `logs-analysis.md` - Datadog Notebooks log analysis template
- `prompt.md` - Additional context for RCA generation

### dd_lib Python Tools

The Claude Agent has access to Python tools for querying Datadog APIs:

| Tool | Description | Parameters |
|------|-------------|------------|
| `get_monitors` | List all monitors | - |
| `get_triggered_monitors` | Get monitors in Alert/Warn state | `limit` |
| `get_host_info` | Get detailed host information | `hostname` |
| `search_logs` | Search logs with query | `query`, `from_time`, `to_time`, `limit` |
| `get_events` | Get events in time range | `from_time`, `to_time` |
| `get_monitor_details` | Get specific monitor config | `monitor_id` |
| `list_services` | List APM services | - |

Tools can be invoked via the `/tools/execute` endpoint or used automatically during RCA.

**Auto-write Mode**: The agent can create new dd_lib functions dynamically via `/tools/create-function`, allowing it to extend its capabilities as needed.

## Development

```bash
# Run tests
cd mkii_ddog_server
make test

# Run specific service tests
go test ./services/user/

# Build
make build
```

## Traffic Generator

Generate realistic API traffic for APM demos with optional failure injection.

```bash
# Start traffic generator (default 10% failure rate)
./scripts/traffic-generator.sh start http://localhost:8080

# Start with custom failure rate
FAILURE_RATE=20 ./scripts/traffic-generator.sh start http://localhost:8080

# Disable failure injection
FAILURE_RATE=0 ./scripts/traffic-generator.sh start http://localhost:8080

# Check status
./scripts/traffic-generator.sh status

# Stop
./scripts/traffic-generator.sh stop

# View logs
tail -f /tmp/rayne-traffic-generator.log
```

### Failure Injection

The traffic generator randomly injects 4xx and 5xx errors to simulate real-world error conditions:

**4xx Client Errors:**
- 404 Not Found (non-existent endpoints, invalid IDs)
- 400 Bad Request (malformed JSON, empty body)
- 405 Method Not Allowed (wrong HTTP methods)

**5xx Server Errors:**
- 500 Internal Server Error (invalid payloads, schema violations)

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `FAILURE_RATE` | 10 | Percentage chance of failure per traffic cycle (0-100) |

## Frontend Traffic Generator

Generate realistic frontend traffic with proper RUM (Real User Monitoring) integration. 25% of simulated visitors are "new users" who receive fresh UUIDs from the backend, while 75% are "returning users" who reuse previously assigned UUIDs.

### Usage

```bash
# Show help
./scripts/frontend-traffic-generator.sh --help

# Start with defaults (25% new users)
./scripts/frontend-traffic-generator.sh start

# Start with 40% new users
./scripts/frontend-traffic-generator.sh -n 40 start

# Start with custom URLs
./scripts/frontend-traffic-generator.sh -f http://frontend:3000 -b http://api:8080 start

# Use environment variables
NEW_USER_RATE=30 ./scripts/frontend-traffic-generator.sh start

# Check status
./scripts/frontend-traffic-generator.sh status

# Stop
./scripts/frontend-traffic-generator.sh stop

# View logs
tail -f /tmp/rayne-frontend-traffic.log
```

### Command-Line Options

| Option | Description |
|--------|-------------|
| `-n, --new-rate PERCENT` | Percentage of new users (default: 25) |
| `-f, --frontend URL` | Frontend server URL (default: http://localhost:3000) |
| `-b, --backend URL` | Backend API URL (default: http://localhost:8080) |
| `-v, --verbose` | Enable verbose logging |
| `-h, --help` | Show help with examples and integration guide |

### How It Works

1. **New Users (default 25%)**: No existing UUID is sent → backend generates new UUID via `/v1/rum/init`
2. **Returning Users (default 75%)**: Existing UUID from pool is reused → backend returns same UUID
3. **Session Tracking**: Each visit creates a new session, even for returning users
4. **Event Simulation**: Page views, navigation, clicks, and session ends are tracked
5. **APM Trace Injection**: Captures `trace_id` and `span_id` from init response and propagates to all track calls

### APM Trace Correlation

The traffic generator automatically captures APM trace context from `/v1/rum/init` responses and propagates it to all subsequent `/v1/rum/track` calls. This allows you to correlate RUM sessions with backend APM traces in Datadog.

**Response from /v1/rum/init:**
```json
{
    "visitor_uuid": "a1b2c3d4-...",
    "session_id": "f9e8d7c6-...",
    "is_new": true,
    "trace_id": "1234567890123456",
    "span_id": "9876543210987654"
}
```

**In Datadog APM:**
- Filter by `@usr.id` to see traces for a specific visitor UUID
- Use `trace_id` to jump from a RUM event directly to the backend APM trace
- View the full request flow from frontend to backend

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `FRONTEND_URL` | http://localhost:3000 | Frontend server URL |
| `BACKEND_URL` | http://localhost:8080 | Backend API URL |
| `NEW_USER_RATE` | 25 | Percentage of visits that are "new users" (0-100) |

### Visitor Pool

Known visitor UUIDs are stored in `/tmp/rayne-visitor-pool.txt` and persist across runs. This simulates realistic returning user behavior. The pool is capped at 100 UUIDs (oldest are removed when exceeded).

### Verifying Traffic

```bash
# Check visitor analytics
curl localhost:8080/v1/rum/analytics

# Check recent sessions (should show mix of new/returning)
curl localhost:8080/v1/rum/sessions

# View unique visitors
curl localhost:8080/v1/rum/visitors?period=1h
```

## Integrating RUM with External Sites

Any website can use Rayne's RUM service for server-generated visitor UUIDs and unique visitor tracking. This provides a centralized, backend-verified system for identifying new vs returning visitors.

### Quick Integration

Add this script to your website's `<body>` (before closing `</body>` tag):

```html
<script>
    window.RAYNE_API_BASE = 'https://your-rayne-server.com';  // Your Rayne backend URL
</script>
<script src="https://your-rayne-server.com/static/js/datadog-rum-init.js"></script>
```

Or host the script locally and configure the API base:

```html
<script>
    window.RAYNE_API_BASE = 'https://api.example.com';
</script>
<script src="/js/rayne-rum.js"></script>
```

### Manual Integration

For full control, implement the RUM flow manually:

#### 1. Initialize Visitor (on page load)

```javascript
async function initVisitor() {
    const storedUuid = localStorage.getItem('rayne_visitor_uuid');
    const storedSession = localStorage.getItem('rayne_session_id');

    const response = await fetch('https://rayne-api.example.com/v1/rum/init', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            visitor_uuid: storedUuid || undefined,  // Omit for new visitors
            session_id: storedSession || undefined,
            user_agent: navigator.userAgent,
            referrer: document.referrer,
            page_url: window.location.href
        })
    });

    const data = await response.json();

    // Store for future visits
    localStorage.setItem('rayne_visitor_uuid', data.visitor_uuid);
    localStorage.setItem('rayne_session_id', data.session_id);

    console.log(data.is_new ? 'New visitor!' : 'Returning visitor');
    return data;
}
```

#### 2. Track Events

```javascript
async function trackEvent(eventType, metadata = {}) {
    const visitorUuid = localStorage.getItem('rayne_visitor_uuid');
    const sessionId = localStorage.getItem('rayne_session_id');

    await fetch('https://rayne-api.example.com/v1/rum/track', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            visitor_uuid: visitorUuid,
            session_id: sessionId,
            event_type: eventType,  // 'view', 'action', 'error', 'resource', 'long_task'
            page_url: window.location.href,
            page_title: document.title,
            metadata: metadata,
            timestamp: new Date().toISOString()
        })
    });
}

// Track page view
trackEvent('view');

// Track custom actions
trackEvent('action', { action_name: 'button_click', button_id: 'signup' });
```

#### 3. End Session (on page unload)

```javascript
window.addEventListener('beforeunload', () => {
    const sessionId = localStorage.getItem('rayne_session_id');
    const sessionStart = localStorage.getItem('rayne_session_start');
    const duration = sessionStart ? Date.now() - parseInt(sessionStart) : 0;

    navigator.sendBeacon('https://rayne-api.example.com/v1/rum/session/end', JSON.stringify({
        session_id: sessionId,
        duration_ms: duration,
        exit_page: window.location.href
    }));
});
```

### API Reference

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/v1/rum/init` | POST | Get/create visitor UUID. Returns `is_new: true` for first-time visitors |
| `/v1/rum/track` | POST | Track events (views, actions, errors) |
| `/v1/rum/session/end` | POST | End a session with duration and exit page |
| `/v1/rum/visitor/{uuid}` | GET | Get visitor details and session history |
| `/v1/rum/analytics` | GET | Get analytics (unique visitors, sessions, event counts) |

### Response Format

**POST /v1/rum/init Response:**

```json
{
    "visitor_uuid": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "session_id": "f9e8d7c6-b5a4-3210-fedc-ba0987654321",
    "is_new": true,
    "message": "Welcome, new visitor!",
    "trace_id": "1234567890123456789",
    "span_id": "9876543210987654321"
}
```

- `is_new: true` - First-time visitor (new UUID generated)
- `is_new: false` - Returning visitor (UUID recognized from database)
- `trace_id` - APM trace ID for correlating RUM sessions with backend traces
- `span_id` - APM span ID for the init request

### APM Trace Correlation

The RUM endpoints return APM trace context (`trace_id`, `span_id`) that allows you to correlate frontend RUM sessions with backend APM traces. Pass these values in subsequent `/v1/rum/track` calls:

```javascript
// After init, store trace context
const { trace_id, span_id } = initResponse;

// Include in track calls for correlation
await fetch('/v1/rum/track', {
    method: 'POST',
    body: JSON.stringify({
        visitor_uuid: visitorUuid,
        session_id: sessionId,
        event_type: 'view',
        trace_id: trace_id,   // Links RUM event to backend trace
        span_id: span_id
    })
});
```

In Datadog, you can then navigate from RUM sessions to APM traces using the trace ID.

### CORS Configuration

If your site is on a different domain than Rayne, ensure Rayne has CORS enabled for your origin. Add your domain to the allowed origins in Rayne's configuration.

### localStorage Keys

| Key | Description |
|-----|-------------|
| `rayne_visitor_uuid` | Persistent visitor UUID (survives browser restart) |
| `rayne_session_id` | Current session ID (new per visit) |
| `rayne_session_start` | Session start timestamp for duration calculation |

## Desktop Notifications

Rayne includes a local notification server that receives webhooks and displays desktop notifications via `notify-send` with Dunst integration for custom styling.

### Setup

```bash
# Start the notification server
./scripts/notify-server.py

# Or with custom port
./scripts/notify-server.py -p 8888

# Or bind to localhost only
./scripts/notify-server.py --bind 127.0.0.1
```

### Dunst Configuration (Orange Border)

Add this rule to `~/.config/dunst/dunstrc` for orange-bordered Rayne alerts:

```ini
[rayne_alert]
    appname = Rayne
    urgency = critical
    background = "#1a1a1a"
    foreground = "#ffffff"
    frame_color = "#ff8c00"
    frame_width = 3
    timeout = 0
```

Reload dunst: `killall dunst; dunst &`

### Webhook Format

```json
{
  "title": "Alert Title",
  "message": "Alert description",
  "urgency": "critical"  // critical, normal, or low
}
```

### Connecting from Kubernetes

From minikube pods, the notification server is accessible at:
```
http://host.minikube.internal:9999
```

## Resource Requirements (Minikube)

| Component | Memory Request | Memory Limit | CPU Request | CPU Limit |
|-----------|---------------|--------------|-------------|-----------|
| Rayne | 64Mi | 256Mi | 100m | 500m |
| Claude Agent | 512Mi | 2Gi | 250m | 1000m |
| PostgreSQL | 128Mi | 512Mi | 100m | 500m |
| Qdrant | 256Mi | 1Gi | 100m | 500m |
| Ollama | 2Gi | 8Gi | 500m | 2000m |

**Recommended minikube config:** `--cpus=4 --memory=12288`

## License

MIT
