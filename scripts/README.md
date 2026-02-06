# Rayne Traffic Generators

This directory contains traffic generators for testing Rayne's RUM (Real User Monitoring) integration with Datadog.

## Headless Browser Generator (Recommended)

Uses Puppeteer to generate real browser traffic with the actual Datadog RUM SDK. This is the recommended approach as it creates authentic browser sessions that appear exactly like real user traffic in Datadog.

### Prerequisites

```bash
cd scripts
npm install
```

### Quick Start

```bash
# Generate 10 sessions (default)
npm run traffic

# Generate 5 sessions with visible browser
npm run traffic:visible

# Run continuously
npm run traffic:continuous

# Show help
npm run traffic:help
```

### Usage

```bash
node headless-traffic-generator.js [OPTIONS]
```

### Options

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `--frontend-url` | `FRONTEND_URL` | http://localhost:3000 | Frontend URL to visit |
| `--backend-url` | `BACKEND_URL` | http://localhost:8080 | Backend URL for RUM init |
| `--sessions` | `SESSIONS` | 10 | Number of sessions to generate |
| `--concurrent` | `CONCURRENT` | 2 | Concurrent browser instances |
| `--new-user-rate` | `NEW_USER_RATE` | 25 | Percentage of new vs returning visitors |
| `--max-pool-size` | `MAX_POOL_SIZE` | 100 | Maximum visitor pool size |
| `--pool-file` | `POOL_FILE` | /tmp/rayne-visitor-pool-headless.json | Visitor pool file path |
| `--min-delay` | `MIN_DELAY` | 2000 | Minimum delay between sessions (ms) |
| `--max-delay` | `MAX_DELAY` | 10000 | Maximum delay between sessions (ms) |
| `--headless` | `HEADLESS=true` | true | Run in headless mode |
| `--no-headless` | `HEADLESS=false` | - | Show browser window |
| `--verbose` | `VERBOSE=true` | false | Show detailed logs |
| `--continuous` | `CONTINUOUS=true` | false | Run until stopped |
| `--help`, `-h` | - | - | Show help message |

### Examples

```bash
# Generate 20 sessions with 50% new users
node headless-traffic-generator.js --sessions 20 --new-user-rate 50

# Run with 4 concurrent browsers
node headless-traffic-generator.js --concurrent 4 --sessions 100

# Continuous mode with verbose output
node headless-traffic-generator.js --continuous --verbose

# Using environment variables
SESSIONS=50 NEW_USER_RATE=30 VERBOSE=true node headless-traffic-generator.js
```

### npm Scripts

| Script | Description |
|--------|-------------|
| `npm run traffic` | Generate 10 sessions (headless) |
| `npm run traffic:help` | Show help message |
| `npm run traffic:verbose` | Generate with verbose logging |
| `npm run traffic:visible` | Generate 3 sessions with visible browser |
| `npm run traffic:continuous` | Run continuously until Ctrl+C |
| `npm run traffic:bulk` | Generate 50 sessions with 4 browsers |

---

## Shell Script Generator (Legacy)

A bash-based traffic generator that sends RUM events directly to the Datadog intake API. This is useful when you can't use Puppeteer or need a lightweight solution.

> **Note**: The headless browser approach is recommended as it generates more authentic traffic that Datadog processes reliably.

### Usage

```bash
# Start in background with Datadog integration
./frontend-traffic-generator.sh -d start

# Stop
./frontend-traffic-generator.sh stop

# Show status
./frontend-traffic-generator.sh status

# View logs
tail -f /tmp/rayne-frontend-traffic.log
```

### Options

| Option | Env Variable | Default | Description |
|--------|--------------|---------|-------------|
| `-d`, `--datadog` | `SEND_TO_DATADOG=true` | false | Send events to Datadog RUM |
| `-v`, `--verbose` | `VERBOSE=true` | false | Verbose output |
| `-f`, `--frontend` | `FRONTEND_URL` | http://localhost:3000 | Frontend URL |
| `-b`, `--backend` | `BACKEND_URL` | http://localhost:8080 | Backend URL |
| `-r`, `--rate` | `NEW_USER_RATE` | 25 | New user percentage |

### Environment Variables

```bash
# Datadog RUM configuration
DD_RUM_APPLICATION_ID=your-app-id
DD_RUM_CLIENT_TOKEN=your-client-token
DD_RUM_SITE=datadoghq.com
DD_RUM_SERVICE=rayne-frontend
DD_RUM_ENV=staging
```

---

## How It Works

Both generators simulate realistic user behavior:

1. **New vs Returning Visitors**: Based on `NEW_USER_RATE`, visitors are either new (get a fresh UUID from the backend) or returning (use a UUID from the visitor pool)

2. **Backend Integration**: Calls `/v1/rum/init` to get visitor UUIDs and session IDs, and `/v1/rum/track` to track events

3. **Realistic Browsing**: Visits multiple pages, scrolls, interacts with elements, and has random delays between actions

4. **Datadog RUM**: The headless generator uses the actual SDK loaded on your frontend page, ensuring authentic RUM sessions appear in Datadog

## Verifying Sessions in Datadog

After running the traffic generator, check your sessions at:

```
https://app.datadoghq.com/rum/sessions?query=@service:rayne-frontend
```

Sessions should appear within 1-2 minutes of being generated.
