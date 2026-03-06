<!-- 
  ██████╗  █████╗ ██╗   ██╗███╗   ██╗███████╗
  ██╔══██╗██╔══██╗╚██╗ ██╔╝████╗  ██║██╔════╝
  ██████╔╝███████║ ╚████╔╝ ██╔██╗ ██║█████╗  
  ██╔══██╗██╔══██║  ╚██╔╝  ██║╚██╗██║██╔══╝  
  ██║  ██║██║  ██║   ██║   ██║ ╚████║███████╗
  ╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═══╝╚══════╝
-->

<div align="center">

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:0D1117,50:FF6B35,100:00D4FF&height=200&section=header&text=rayne&fontSize=80&fontColor=ffffff&animation=fadeIn&fontAlignY=35&desc=AI-Powered%20Observability%20Gateway&descAlignY=55&descSize=20" width="100%"/>

<p>
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/🐕_Datadog-632CA6?style=for-the-badge" alt="Datadog"/>
  <img src="https://img.shields.io/badge/Claude-AI_Powered-FF6B35?style=for-the-badge&logo=anthropic&logoColor=white" alt="Claude"/>
  <img src="https://img.shields.io/badge/Kubernetes-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white" alt="K8s"/>
</p>

<p>
  <a href="#quick-start">Quick Start</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#features">Features</a> •
  <a href="#api-reference">API</a> •
  <a href="#deployment">Deploy</a>
</p>

</div>

---

## `> cat /etc/rayne.conf` 📋

```bash
+-------------------------------------------------------------------------+
|                                                                         |
|  DESCRIPTION="Datadog API Gateway with AI-Powered Root Cause Analysis"  |
|  VERSION="2.0"                                                          |
|  LICENSE="MIT"                                                          |
|                                                                         |
|  # Wraps Datadog API for downtimes, events, hosts, webhooks, and RUM    |
|  # Automatic RCA via Claude when alerts fire                            |
|  # Vector DB storage for learning from past incidents                   |
|                                                                         |
+-------------------------------------------------------------------------+
```

---

## `> ./rayne --features` ⚡ {#features}

<div align="center">

| 🔌 **Integrations** | 🤖 **AI Analysis** | 📊 **Observability** |
|:---:|:---:|:---:|
| Multi-Account Datadog | Claude-Powered RCA | RUM Visitor Tracking |
| Webhook Processing | Vector DB Learning | Desktop Notifications |
| Auto-Downtime Triggers | Incident Notebooks | LLM Observability |

</div>

**Core Capabilities:**
- 🏢 **Multi-Account Support** — Manage US Gov, Commercial, EU orgs from one instance
- 🎯 **Webhook Engine** — Receive, store, process webhooks with auto-routing
- 🧠 **AI Root Cause Analysis** — Claude analyzes alerts with pre-fetched context
- 📓 **Auto-Generated Notebooks** — Datadog Notebooks with hyperlinked resources
- 🔍 **Pattern Matching** — Qdrant + Ollama for similar incident detection
- 🐍 **dd_lib Tools** — Extensible Python tools with auto-write capability
- 🖥️ **RUM Tracking** — Server-generated UUIDs and session analytics

---

## `> rayne architecture --map` 🗺️ {#architecture}

<div align="center">

```
                        +---------------------------------------+
                        |          R A Y N E   S Y S T E M      |
                        +---------------------------------------+

                                 +-----------------+
                                 |     DATADOG     |
                                 |     WEBHOOKS    |
                                 +--------+--------+
                                          |
                                          v
+------------------------------------------------------------------------+
|                              RAYNE POD                                 |
|                                                                        |
|   +------------------------+         +------------------------+        |
|   |    RAYNE GO API        |         |  CLAUDE AGENT SIDECAR  |        |
|   |      Port 8080         | <---->  |      Port 9000         |        |
|   |                        |  HTTP   |                        |        |
|   |  - Webhooks            |         |  - Claude Code         |        |
|   |  - Accounts            |         |  - dd_lib Python       |        |
|   |  - RUM Tracking        |         |  - Assets/Templates    |        |
|   |  - Events/Hosts        |         |                        |        |
|   |  - Downtimes           |         |                        |        |
|   +------------------------+         +------------------------+        |
|                                                                        |
+------------------------------------------------------------------------+
         |              |                      |               |
         v              v                      v               v
  +------------+ +------------+        +------------+  +------------+
  |  Postgres  | |  Datadog   |        |   Qdrant   |  |   Ollama   |
  |   :5432    | |    APIs    |        |  Vectors   |  | Embeddings |
  +------------+ +------------+        +------------+  +------------+
     Storage       External              RCA Store       Local LLM
```

</div>

---

## `> rayne start --quick` 🚀 {#quick-start}

<details>
<summary><b>🐳 Docker Compose (Recommended)</b></summary>

```bash
# Clone & configure
git clone https://github.com/Nokodoko/rayne.git && cd rayne
export DD_API_KEY=your-datadog-api-key
export DD_APP_KEY=your-datadog-app-key

# Launch
docker-compose up -d --build

# Verify
curl localhost:8080/health

# Seed demo data
curl -X POST localhost:8080/v1/demo/seed/all
```

</details>

<details>
<summary><b>💻 Local Development</b></summary>

```bash
cd mkii_ddog_server

# Environment
export DD_API_KEY=your-api-key
export DD_APP_KEY=your-app-key
export DB_HOST=localhost DB_PORT=5432
export DB_USER=rayne DB_PASSWORD=raynepassword DB_NAME=rayne

# Run
make r
```

</details>

<details>
<summary><b>☸️ Minikube (Full Stack)</b></summary>

```bash
# Configure
export TF_VAR_ecco_dd_api_key=your-datadog-api-key
export TF_VAR_ecco_dd_app_key=your-datadog-app-key
export ANTHROPIC_API_KEY=your-anthropic-api-key

# Deploy (requires ~12GB RAM)
./scripts/minikube-setup.sh

# Test webhook with RCA
curl -X POST $(minikube service rayne-service --url)/v1/webhooks/receive \
  -H "Content-Type: application/json" \
  -d '{"monitor_id":12345,"alert_status":"Alert","hostname":"web-01"}'
```

</details>

---

## `> rayne api --endpoints` 📡 {#api-reference}

<div align="center">

### 🏥 Health & Auth
| Method | Endpoint | Description |
|:------:|----------|-------------|
| `GET` | `/health` | Health check |
| `POST` | `/login` | User login |
| `POST` | `/register` | User registration |

### 🐕 Datadog Proxies
| Method | Endpoint | Description |
|:------:|----------|-------------|
| `GET` | `/v1/downtimes` | List downtimes |
| `GET` | `/v1/events` | List events |
| `GET` | `/v1/hosts` | List hosts |
| `GET` | `/v1/hosts/active` | Active host count |
| `GET` | `/v1/hosts/{hostname}/tags` | Host tags |

### 🪝 Webhooks
| Method | Endpoint | Description |
|:------:|----------|-------------|
| `POST` | `/v1/webhooks/receive` | Receive webhook *(triggers RCA on Alert/Warn)* |
| `POST` | `/v1/webhooks/receive/{account}` | Receive with explicit routing |
| `GET` | `/v1/webhooks/events` | List stored events |
| `GET` | `/v1/webhooks/stats` | Statistics |

### 🏢 Multi-Account
| Method | Endpoint | Description |
|:------:|----------|-------------|
| `GET` | `/v1/accounts` | List all accounts |
| `POST` | `/v1/accounts` | Create account |
| `POST` | `/v1/accounts/{name}/default` | Set default |
| `POST` | `/v1/accounts/{name}/test` | Test credentials |

### 📊 RUM Tracking
| Method | Endpoint | Description |
|:------:|----------|-------------|
| `POST` | `/v1/rum/init` | Initialize visitor (generates UUID) |
| `POST` | `/v1/rum/track` | Track events |
| `POST` | `/v1/rum/session/end` | End session |
| `GET` | `/v1/rum/analytics` | Get analytics |

### 🤖 Claude Agent (Port 9000)
| Method | Endpoint | Description |
|:------:|----------|-------------|
| `GET` | `/health` | Agent health |
| `POST` | `/analyze` | Trigger RCA analysis |
| `POST` | `/generate-notebook` | Create Datadog notebook |
| `GET` | `/tools` | List dd_lib tools |
| `POST` | `/tools/execute` | Execute tool |

</div>

---

## `> rayne rca --flow` 🧠

<div align="center">

```
                    +-------------------------------------+
                    |      ROOT CAUSE ANALYSIS FLOW       |
                    +-------------------------------------+

+----------+    +----------+    +----------+    +----------+    +----------+
| WEBHOOK  |--->|  STORE   |--->| PREFETCH |--->| ANALYZE  |--->| NOTEBOOK |
| RECEIVE  |    | POSTGRES |    | DD DATA  |    |  CLAUDE  |    |  CREATE  |
+----------+    +----------+    +----------+    +----------+    +----------+
     |                               |               |               |
     |                               v               v               v
     |                         +----------+    +----------+    +----------+
     |                         |   LOGS   |    |  SIMILAR |    | HYPERLINK|
     |                         |  EVENTS  |    |   RCAs   |    |   ALL    |
     |                         |   HOSTS  |    | (Qdrant) |    | RESOURCES|
     |                         +----------+    +----------+    +----------+
     |
     +----------------------------------------------------------------------->
                          Alert/Warn triggers full pipeline
```

</div>

**What happens when an alert fires:**

1. **Webhook Received** — Rayne stores payload in PostgreSQL
2. **Pre-fetch Context** — Error logs, host info, recent events, monitor config
3. **Embed & Search** — Ollama generates embeddings → Qdrant finds similar past incidents
4. **Claude Analysis** — Full context + dd_lib tools + past RCAs
5. **Notebook Created** — Datadog Notebook with hyperlinks to all resources
6. **Store for Learning** — Analysis saved to Qdrant for future pattern matching

---

## `> cat webhook_payload.json` 📨

<details>
<summary><b>Standard Datadog Fields</b></summary>

| Field | Type | Description |
|-------|------|-------------|
| `alert_id` | int64 | Unique alert identifier |
| `alert_status` | string | `"Alert"`, `"OK"`, `"Warn"`, `"No Data"` |
| `monitor_id` | int64 | Datadog monitor ID |
| `hostname` | string | Affected hostname |
| `service` | string | Affected service |
| `tags` | string[] | `["env:production", "team:platform"]` |
| `org_id` | int64 | Organization ID (for multi-account routing) |

</details>

<details>
<summary><b>Example Payload</b></summary>

```json
{
  "alert_id": 123456789,
  "alert_title": "High CPU Usage on web-server-01",
  "alert_status": "Alert",
  "monitor_id": 12345678,
  "hostname": "web-server-01.prod.example.com",
  "service": "api-gateway",
  "tags": ["env:production", "team:platform"],
  "APPLICATION_TEAM": "platform-engineering",
  "URGENCY": "high"
}
```

</details>

---

## `> rayne rum --integration` 📊

<div align="center">

```
                    +-----------------------------------------+
                    |          RUM INTEGRATION FLOW           |
                    +-----------------------------------------+

     BROWSER                      RAYNE (8080)                DATADOG
        |                              |                          |
        |  1. Load DD_RUM SDK          |                          |
        |  2. POST /v1/rum/init ------>|                          |
        |  <---- visitor_uuid, session |                          |
        |  3. DD_RUM.setUser({id})     |                          |
        |  4. RUM events --------------+------------------------->|
        |     (@usr.id = our uuid)     |                          |
        |  5. POST /v1/rum/track ----->|                          |
        |                              |  PostgreSQL              |
```

</div>

**Quick Integration:**
```html
<script>
    window.RAYNE_API_BASE = 'https://your-rayne-server.com';
</script>
<script src="https://your-rayne-server.com/static/js/datadog-rum-init.js"></script>
```

---

## `> rayne env --vars` ⚙️

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `DD_API_KEY` | ✅ | - | Datadog API key |
| `DD_APP_KEY` | ✅ | - | Datadog Application key |
| `DD_API_URL` | ❌ | `https://api.ddog-gov.com` | Datadog API URL |
| `ANTHROPIC_API_KEY` | ❌ | - | For Claude RCA |
| `DB_HOST` | ❌ | `localhost` | PostgreSQL host |
| `CLAUDE_AGENT_URL` | ❌ | `http://localhost:9000` | Sidecar URL |
| `QDRANT_URL` | ❌ | `http://qdrant-service:6333` | Vector DB |
| `OLLAMA_URL` | ❌ | `http://ollama-service:11434` | Embeddings |

### Datadog Regions

| Region | Base URL |
|--------|----------|
| 🇺🇸 US Gov | `https://api.ddog-gov.com` |
| 🇺🇸 US Commercial | `https://api.datadoghq.com` |
| 🇪🇺 EU | `https://api.datadoghq.eu` |
| 🌏 AP1 | `https://api.ap1.datadoghq.com` |

---

## `> tree rayne/` 📁

```
rayne/
├── mkii_ddog_server/          # 🦫 Go server
│   ├── cmd/                   #    Entry point
│   └── services/              #    Service handlers
│       ├── accounts/          #    Multi-account management
│       ├── webhooks/          #    Webhook processing + Claude
│       ├── rum/               #    RUM tracking
│       └── ...
├── docker/
│   └── claude-agent/          # 🤖 Claude Agent sidecar
├── dd_lib/                    # 🐍 Python Datadog tools
├── assets/                    # 📝 Incident templates
├── k8s/                       # ☸️  Kubernetes manifests
├── helm/                      # ⎈  Helm values
└── scripts/                   # 🔧 Utilities
    ├── minikube-setup.sh
    ├── notify-server.py
    └── traffic-generator.sh
```

---

## `> rayne resources --minikube` 💾

| Component | Memory | CPU |
|-----------|--------|-----|
| Rayne | 64Mi - 256Mi | 100m - 500m |
| Claude Agent | 512Mi - 2Gi | 250m - 1000m |
| PostgreSQL | 128Mi - 512Mi | 100m - 500m |
| Qdrant | 256Mi - 1Gi | 100m - 500m |
| Ollama | 2Gi - 8Gi | 500m - 2000m |

**Recommended:** `minikube start --cpus=4 --memory=12288`

---

<div align="center">

<img src="https://capsule-render.vercel.app/api?type=waving&color=0:0D1117,50:FF6B35,100:00D4FF&height=100&section=footer" width="100%"/>

<p>
  <img src="https://img.shields.io/badge/License-MIT-00D4FF?style=flat-square" alt="License"/>
  <img src="https://img.shields.io/badge/PRs-Welcome-00FF9F?style=flat-square" alt="PRs Welcome"/>
</p>

<sub>Built with 🦫 Go • 🤖 Claude • ☕ Caffeine</sub>

</div>
