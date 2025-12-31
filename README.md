# Rayne

A Go-based REST API server that wraps the Datadog API, providing endpoints to manage downtimes, events, hosts, webhooks, and RUM (Real User Monitoring) visitor tracking. Designed as a demo environment for showcasing Datadog monitoring capabilities.

## Features

- **Datadog API Proxy**: Downtimes, events, hosts, and private location management
- **Webhook Handling**: Receive, store, and process Datadog webhooks with auto-downtime triggers
- **RUM Visitor Tracking**: Server-generated UUIDs, session management, and analytics
- **Demo Data Generators**: Seed fake data for demonstrations
- **Kubernetes Ready**: Includes manifests for minikube deployment

## Prerequisites

- Go 1.22+ (for local development)
- Docker & Docker Compose (recommended)
- PostgreSQL 16+ (if running locally without Docker)
- Datadog API and Application keys

## Quick Start

### Docker Compose (Recommended)

```bash
# Clone the repository
git clone https://github.com/Nokodoko/rayne.git
cd rayne

# Set your Datadog API keys
export DD_API_KEY=your-api-key
export DD_APP_KEY=your-app-key

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

### Minikube Deployment

```bash
# Edit k8s/datadog-secrets.yaml with your API keys
vim k8s/datadog-secrets.yaml

# Run the setup script
./scripts/minikube-setup.sh
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
| POST | `/v1/webhooks/receive` | Receive Datadog webhooks |
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

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DD_API_KEY` | Yes | - | Datadog API key |
| `DD_APP_KEY` | Yes | - | Datadog Application key |
| `DB_HOST` | No | localhost | PostgreSQL host |
| `DB_PORT` | No | 5432 | PostgreSQL port |
| `DB_USER` | No | - | PostgreSQL user |
| `DB_PASSWORD` | No | - | PostgreSQL password |
| `DB_NAME` | No | - | PostgreSQL database |

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
│   │   └── webhooks/     # Webhook handling
│   └── Dockerfile
├── k8s/                  # Kubernetes manifests
├── scripts/              # Deployment scripts
└── docker-compose.yaml
```

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

## License

MIT
rayne
