# Rayne

A Go-based REST API server that wraps the Datadog API, providing endpoints to manage downtimes, events, hosts, webhooks, and RUM (Real User Monitoring) visitor tracking. Features AI-powered Root Cause Analysis using Claude Code with vector database storage for learning from past incidents.

## Features

- **Datadog API Proxy**: Downtimes, events, hosts, and private location management
- **Webhook Handling**: Receive, store, and process Datadog webhooks with auto-downtime triggers
- **AI-Powered RCA**: Automatic Root Cause Analysis via Claude Code when alerts trigger
- **Vector DB Storage**: Store and retrieve past RCAs using Qdrant for pattern matching
- **Local Embeddings**: Generate embeddings with Ollama (Gemma 2B) for similarity search
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

# Test webhook with RCA trigger
curl -X POST $(minikube service rayne-service --url)/v1/webhooks/receive \
  -H "Content-Type: application/json" \
  -d '{"monitor_id": 123, "monitor_name": "CPU Alert", "alert_status": "Alert", "scope": "host:server-1"}'
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
| GET | `/v1/webhooks/events` | List stored webhook events |
| POST | `/v1/webhooks/create` | Create webhook in Datadog |
| GET | `/v1/webhooks/stats` | Webhook statistics |
| POST | `/v1/webhooks/config` | Save webhook configuration |

### RUM Visitor Tracking
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/rum/init` | Initialize visitor (generates UUID) |
| POST | `/v1/rum/track` | Track RUM events |
| POST | `/v1/rum/session/end` | End a session |
| GET | `/v1/rum/visitors` | Get unique visitor count |
| GET | `/v1/rum/analytics` | Get visitor analytics |
| GET | `/v1/rum/sessions` | List recent sessions |

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
| POST | `/analyze` | Trigger RCA analysis for an alert |
| POST | `/generate-notebook` | Generate Datadog notebook from analysis |
| GET | `/templates` | List available incident report templates |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DD_API_KEY` | Yes | - | Datadog API key |
| `DD_APP_KEY` | Yes | - | Datadog Application key |
| `ANTHROPIC_API_KEY` | Yes* | - | Anthropic API key for Claude Code (*required for RCA) |
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
│   │   ├── demo/         # Demo data generators
│   │   ├── downtimes/    # Datadog downtimes
│   │   ├── events/       # Datadog events
│   │   ├── hosts/        # Datadog hosts
│   │   ├── pl/           # Private locations
│   │   ├── rum/          # RUM tracking
│   │   ├── user/         # User management
│   │   └── webhooks/     # Webhook handling + Claude invocation
│   └── Dockerfile
├── docker/
│   └── claude-agent/     # Claude Agent sidecar
│       ├── Dockerfile    # Node.js + Python + Claude Code CLI
│       └── agent-server.js # HTTP wrapper for Claude Code
├── dd_lib/               # Python Datadog tools (baked into sidecar)
├── assets/               # Incident report templates
├── k8s/                  # Kubernetes manifests
│   ├── rayne-deployment.yaml      # Rayne + Claude Agent sidecar
│   ├── postgres-deployment.yaml   # PostgreSQL
│   ├── qdrant-deployment.yaml     # Qdrant vector DB
│   ├── ollama-deployment.yaml     # Ollama (Gemma 2B)
│   ├── assets-configmap.yaml      # Incident templates
│   └── anthropic-secrets.yaml     # Anthropic API key
├── helm/                 # Helm values for Datadog Agent
├── scripts/              # Deployment scripts
└── docker-compose.yaml
```

## AI-Powered Root Cause Analysis

When a webhook arrives with `alert_status: "Alert"` or `alert_status: "Warn"`:

1. **Rayne** receives the webhook and stores it in PostgreSQL
2. **Claude Agent** is invoked with the alert details
3. **Ollama** generates embeddings for the alert to find similar past incidents
4. **Qdrant** searches for similar RCAs from previous incidents
5. **Claude Code** analyzes the alert with context from:
   - Past similar RCAs
   - Python dd_lib tools for Datadog API access
   - Incident report templates from assets/
6. The analysis is stored back in Qdrant for future reference

### Templates Available

- `incident-report-cloned.md` - Full incident report template with sections for Summary, Outage Tracking, Response Breakdown, and Organizational Analysis
- `logs-analysis.md` - Datadog Notebooks log analysis template
- `prompt.md` - Additional context for RCA generation

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
