# Plan: Self-Healing Error Recovery & Webhook Failure Alerting

## Overview

Two interconnected features to close the observability gap in the RCA pipeline. Today, when `invokeClaudeCode()` fails (expired OAuth token, network timeout, Qdrant down), errors are logged to stdout but no external system is notified. The user discovered missed RCAs only via desktop notifications; teams relying on RCA notebooks had zero visibility. These features ensure that (1) the agent sidecar retries transient failures and auto-refreshes expired credentials, and (2) when all retries are exhausted, a Datadog event is created so monitors can fire and teams get paged.

### Scope

**Feature 1 -- Webhook Failure Alerting (Go server + Node sidecar)**
- When the RCA pipeline fails after retries, create a Datadog event via `POST /api/v1/events`
- Optionally create a minimal "failure notebook" in Datadog documenting what went wrong
- Send desktop notification with failure context (already partially works via `sendDesktopNotification`)
- Structured JSON logging for all failure paths (machine-parseable for Datadog log monitoring)

**Feature 2 -- Self-Healing Error Recovery (Node sidecar)**
- OAuth token auto-refresh: detect expired/401 from Claude CLI, use `refreshToken` from `~/.claude/.credentials.json` to obtain new `accessToken`, retry
- Exponential backoff with jitter for transient errors (ECONNREFUSED, ETIMEDOUT, EPIPE, spawn failures)
- 429 rate limit handling: parse `Retry-After` header, wait, retry
- Error classification: categorize errors into `auth`, `network`, `rate_limit`, `unknown` for routing to the correct recovery strategy

### Architecture Decision

**Retry and token refresh logic lives in the Node.js sidecar (`agent-server.js`), NOT in the Go server.**

Rationale:
- The sidecar owns the Claude CLI subprocess lifecycle and has direct access to `~/.claude/.credentials.json`
- The Go server's `ClaudeAgent.invokeAnalysis()` already treats the sidecar as a black box (POST to `/analyze`, get result)
- Putting retry in the sidecar keeps the Go orchestrator simple: it fires one HTTP request and either gets a result or an error
- The sidecar already has `httpRequest()` for Datadog API calls, so creating events/notebooks from JS is natural

**Failure alerting is dual-layer:**
1. **Sidecar-side (primary):** After retries exhaust, the sidecar creates a Datadog event and sends desktop notification BEFORE returning the error HTTP response to the Go server. This ensures alerting happens even if the Go server has its own issues.
2. **Go-side (secondary):** The webhook `ProcessorOrchestrator` checks `OrchestratorResult.Errors` and, if agent analysis failed, posts a supplementary Datadog event with Go-side context (event ID, processing duration, which tier succeeded/failed).

### Key Files

| File | Changes |
|------|---------|
| `docker/claude-agent/agent-server.js` | Add: `classifyError()`, `retryWithBackoff()`, `refreshOAuthToken()`, `createFailureEvent()`, `createFailureNotebook()`, `structuredLog()`. Modify: `invokeClaudeCode()`, `invokeClaudeCodeCLI()`, `/analyze` handler, `/watchdog` handler |
| `mkii_ddog_server/services/agents/orchestrator.go` | Add: `reportFailureToDatadog()` method on `AgentOrchestrator`. Modify: `Analyze()` to call failure reporter on error |
| `mkii_ddog_server/services/agents/claude_agent.go` | Add structured error context to `invokeAnalysis()` return, parse sidecar error responses for error_type |
| `mkii_ddog_server/services/agents/failure_alerter.go` | New file: `FailureAlerter` struct that posts Datadog events via `requests.Post` |
| `mkii_ddog_server/services/webhooks/orchestrator.go` | Modify: `Process()` to invoke failure alerter when Tier 2 fails |

### Credentials Structure (for token refresh implementation)

The file at `~/.claude/.credentials.json` (inside container: `/home/node/.claude/.credentials.json`) contains:

```json
{
  "claudeAiOauth": {
    "accessToken": "sk-ant-oat01-...",
    "refreshToken": "sk-ant-ort01-...",
    "expiresAt": 1770978731359,
    "scopes": ["user:inference", "user:profile", "user:sessions:claude_code"],
    "subscriptionType": "max",
    "rateLimitTier": "default_claude_max_20x"
  }
}
```

The refresh endpoint is `https://console.anthropic.com/v1/oauth/token` (standard OAuth2 refresh grant). The `expiresAt` field is epoch milliseconds; the sidecar should check `Date.now() > expiresAt - 300000` (5 min buffer) to proactively refresh before expiry.

### Error Classification Matrix

| Error Signal | Category | Recovery Strategy | Max Retries |
|---|---|---|---|
| Claude CLI exit code 1 + stderr contains "auth", "token", "401", "403", "unauthorized" | `auth` | Refresh OAuth token, rewrite credentials.json, retry | 2 |
| Claude CLI exit code 1 + stderr contains "rate limit", "429", "too many" | `rate_limit` | Parse Retry-After or wait 60s, retry | 3 |
| spawn ECONNREFUSED, ETIMEDOUT, EPIPE, ENOTFOUND | `network` | Exponential backoff (1s, 2s, 4s, 8s) with jitter | 4 |
| SDK 429 response | `rate_limit` | Wait Retry-After header duration, retry | 3 |
| SDK 401/403 response | `auth` | For SDK: API key is static, no refresh possible; fail immediately | 0 |
| Everything else | `unknown` | Log structured error, no retry | 0 |

---

## Team Members

- Name: sidecar-resilience-engineer
- Role: Implement retry logic, error classification, token refresh, and structured logging in agent-server.js
- Capabilities: Node.js, child_process, OAuth2 refresh flow, exponential backoff patterns

- Name: alerting-engineer
- Role: Implement Datadog event creation and failure notebook generation in both the Node sidecar and Go server
- Capabilities: Datadog API (POST /api/v1/events, POST /api/v1/notebooks), Go HTTP clients, desktop notification integration

- Name: integration-tester
- Role: Write test scenarios, verify end-to-end error flows, validate structured log format
- Capabilities: curl testing, log parsing, mock failure injection

---

## Step by Step Tasks

### Phase 1: Error Classification & Structured Logging (sidecar-resilience-engineer)

**Task 1.1: Add structured logging utility**

File: `docker/claude-agent/agent-server.js`

Add a `structuredLog(level, event, data)` function near the top of the file (after the config constants, around line 120). This replaces ad-hoc `console.log` and `console.error` calls in error paths with machine-parseable JSON.

```javascript
function structuredLog(level, event, data = {}) {
    const entry = {
        timestamp: new Date().toISOString(),
        level,        // 'info', 'warn', 'error'
        event,        // 'claude_invoke_start', 'claude_invoke_error', 'token_refresh', etc.
        service: 'claude-agent-sidecar',
        ...data
    };
    const output = JSON.stringify(entry);
    if (level === 'error') {
        console.error(output);
    } else {
        console.log(output);
    }
}
```

Fields to always include: `timestamp`, `level`, `event`, `service`. Contextual fields: `monitor_id`, `monitor_name`, `error_type` (auth/network/rate_limit/unknown), `error_message`, `retry_count`, `max_retries`, `duration_ms`, `auth_method`.

This enables Datadog log pipelines to parse `event` and `error_type` as facets, and create monitors like "alert if error_type:auth count > 0 in 5m".

**Task 1.2: Add error classification function**

File: `docker/claude-agent/agent-server.js`

Add `classifyError(err, stderr)` function after `structuredLog`. This inspects error messages and stderr output from Claude CLI to categorize the error.

```javascript
function classifyError(err, stderr = '') {
    const msg = (err.message || '').toLowerCase();
    const errOutput = (stderr || '').toLowerCase();
    const combined = msg + ' ' + errOutput;

    if (combined.includes('401') || combined.includes('403') ||
        combined.includes('unauthorized') || combined.includes('token') ||
        combined.includes('auth') || combined.includes('credential') ||
        combined.includes('expired') || combined.includes('invalid session')) {
        return 'auth';
    }

    if (combined.includes('429') || combined.includes('rate limit') ||
        combined.includes('too many requests') || combined.includes('retry-after')) {
        return 'rate_limit';
    }

    if (combined.includes('econnrefused') || combined.includes('etimedout') ||
        combined.includes('epipe') || combined.includes('enotfound') ||
        combined.includes('econnreset') || combined.includes('network') ||
        combined.includes('socket hang up') || combined.includes('dns')) {
        return 'network';
    }

    return 'unknown';
}
```

Returns one of: `'auth'`, `'rate_limit'`, `'network'`, `'unknown'`.

**Task 1.3: Modify `invokeClaudeCodeCLI` to capture stderr for classification**

File: `docker/claude-agent/agent-server.js`, lines 1242-1282

Currently `invokeClaudeCodeCLI` rejects with `new Error(...)` but does not expose the stderr separately. Modify the rejection to include structured metadata:

```javascript
claude.on('close', code => {
    if (code === 0) {
        resolve(stdout);
    } else {
        const err = new Error(`Claude CLI exited with code ${code}: ${stderr}`);
        err.exitCode = code;
        err.stderr = stderr;
        err.errorType = classifyError(err, stderr);
        reject(err);
    }
});

claude.on('error', err => {
    err.errorType = classifyError(err, '');
    reject(err);
});
```

This enriches the error object so the retry wrapper can route to the correct recovery strategy without re-parsing.

---

### Phase 2: OAuth Token Refresh (sidecar-resilience-engineer)

**Task 2.1: Implement `refreshOAuthToken()` function**

File: `docker/claude-agent/agent-server.js`

Add after `getAuthMethod()` (around line 119). This function reads the refresh token from credentials.json, calls the Anthropic OAuth2 token endpoint, and writes the new access token back.

```javascript
async function refreshOAuthToken() {
    structuredLog('info', 'token_refresh_start', {});

    const credsRaw = fs.readFileSync(CLAUDE_CREDS_PATH, 'utf8');
    const creds = JSON.parse(credsRaw);
    const oauth = creds.claudeAiOauth;

    if (!oauth || !oauth.refreshToken) {
        throw new Error('No refresh token available in credentials.json');
    }

    // Call Anthropic OAuth2 token endpoint
    const tokenUrl = 'https://console.anthropic.com/v1/oauth/token';
    const body = JSON.stringify({
        grant_type: 'refresh_token',
        refresh_token: oauth.refreshToken,
        // client_id may be needed - check Claude CLI source if this fails
    });

    return new Promise((resolve, reject) => {
        const urlObj = new URL(tokenUrl);
        const options = {
            hostname: urlObj.hostname,
            port: 443,
            path: urlObj.pathname,
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Content-Length': Buffer.byteLength(body)
            }
        };

        const req = https.request(options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                if (res.statusCode === 200) {
                    try {
                        const tokens = JSON.parse(data);
                        // Update credentials.json with new tokens
                        creds.claudeAiOauth.accessToken = tokens.access_token;
                        if (tokens.refresh_token) {
                            creds.claudeAiOauth.refreshToken = tokens.refresh_token;
                        }
                        creds.claudeAiOauth.expiresAt = Date.now() + (tokens.expires_in * 1000);

                        fs.writeFileSync(CLAUDE_CREDS_PATH, JSON.stringify(creds, null, 2));
                        structuredLog('info', 'token_refresh_success', {
                            expires_at: creds.claudeAiOauth.expiresAt
                        });
                        resolve(true);
                    } catch (e) {
                        reject(new Error(`Failed to parse token response: ${e.message}`));
                    }
                } else {
                    reject(new Error(`Token refresh failed with status ${res.statusCode}: ${data}`));
                }
            });
        });

        req.on('error', (e) => reject(new Error(`Token refresh network error: ${e.message}`)));
        req.write(body);
        req.end();
    });
}
```

**IMPORTANT IMPLEMENTATION NOTE:** The exact OAuth2 endpoint and request body format must be verified by inspecting the Claude CLI source or testing manually. The `claude login` flow uses `claudeAiOauth` provider. If the endpoint requires a `client_id`, it can be extracted from the Claude CLI binary or reverse-engineered from the login flow. Document this as a potential blocker and test with:
```bash
curl -X POST https://console.anthropic.com/v1/oauth/token \
  -H "Content-Type: application/json" \
  -d '{"grant_type":"refresh_token","refresh_token":"sk-ant-ort01-..."}'
```

**Task 2.2: Add proactive token expiry check**

File: `docker/claude-agent/agent-server.js`

Add `isTokenExpiringSoon()` function that checks `expiresAt` with a 5-minute buffer. Call this before invoking Claude CLI to refresh proactively rather than waiting for failure.

```javascript
function isTokenExpiringSoon() {
    try {
        const creds = JSON.parse(fs.readFileSync(CLAUDE_CREDS_PATH, 'utf8'));
        const expiresAt = creds.claudeAiOauth?.expiresAt;
        if (!expiresAt) return false;
        // Refresh if within 5 minutes of expiry
        return Date.now() > (expiresAt - 5 * 60 * 1000);
    } catch {
        return false;
    }
}
```

---

### Phase 3: Retry with Backoff (sidecar-resilience-engineer)

**Task 3.1: Implement `retryWithBackoff()` wrapper**

File: `docker/claude-agent/agent-server.js`

Add a generic retry wrapper that takes an async function and applies error-type-aware retry logic.

```javascript
async function retryWithBackoff(fn, context = {}) {
    const RETRY_CONFIG = {
        auth:       { maxRetries: 2, baseDelay: 1000 },
        rate_limit: { maxRetries: 3, baseDelay: 60000 },
        network:    { maxRetries: 4, baseDelay: 1000 },
        unknown:    { maxRetries: 0, baseDelay: 0 }
    };

    let lastError;

    for (let attempt = 0; attempt <= Math.max(...Object.values(RETRY_CONFIG).map(c => c.maxRetries)); attempt++) {
        try {
            // Proactive token refresh before first attempt
            if (attempt === 0 && getAuthMethod() === 'token' && isTokenExpiringSoon()) {
                structuredLog('info', 'proactive_token_refresh', context);
                await refreshOAuthToken();
            }
            return await fn();
        } catch (err) {
            lastError = err;
            const errorType = err.errorType || classifyError(err, err.stderr || '');
            const config = RETRY_CONFIG[errorType] || RETRY_CONFIG.unknown;

            structuredLog('warn', 'claude_invoke_retry', {
                ...context,
                error_type: errorType,
                error_message: err.message,
                attempt: attempt + 1,
                max_retries: config.maxRetries
            });

            if (attempt >= config.maxRetries) {
                break; // Exhausted retries for this error type
            }

            // Error-type-specific recovery
            if (errorType === 'auth') {
                try {
                    await refreshOAuthToken();
                    structuredLog('info', 'token_refresh_after_auth_error', context);
                } catch (refreshErr) {
                    structuredLog('error', 'token_refresh_failed', {
                        ...context,
                        error_message: refreshErr.message
                    });
                    break; // Can't recover without valid token
                }
            } else if (errorType === 'rate_limit') {
                // Parse Retry-After if available, otherwise use baseDelay
                const retryAfter = parseRetryAfter(err) || config.baseDelay;
                structuredLog('info', 'rate_limit_wait', { ...context, wait_ms: retryAfter });
                await sleep(retryAfter);
                continue; // Skip the exponential backoff below
            }

            // Exponential backoff with jitter for network errors
            const delay = config.baseDelay * Math.pow(2, attempt) + Math.random() * 1000;
            structuredLog('info', 'backoff_wait', { ...context, wait_ms: Math.round(delay) });
            await sleep(delay);
        }
    }

    // All retries exhausted
    lastError.retriesExhausted = true;
    lastError.errorType = lastError.errorType || classifyError(lastError, lastError.stderr || '');
    throw lastError;
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

function parseRetryAfter(err) {
    // Try to extract Retry-After from error message or headers
    const match = (err.message || '').match(/retry.after[:\s]*(\d+)/i);
    if (match) return parseInt(match[1], 10) * 1000;
    return null;
}
```

**Task 3.2: Wrap `invokeClaudeCode()` with retry**

File: `docker/claude-agent/agent-server.js`, lines 1226-1239

Modify `invokeClaudeCode()` to use the retry wrapper:

```javascript
async function invokeClaudeCode(prompt, workDir = WORK_DIR, context = {}) {
    return retryWithBackoff(async () => {
        const authMethod = getAuthMethod();
        structuredLog('info', 'claude_invoke_start', {
            ...context,
            auth_method: authMethod
        });

        if (authMethod === 'token') {
            return invokeClaudeCodeCLI(prompt, workDir);
        } else if (authMethod === 'apikey') {
            return invokeClaudeCodeSDK(prompt);
        } else {
            const err = new Error('No valid Claude authentication');
            err.errorType = 'auth';
            throw err;
        }
    }, context);
}
```

The `context` parameter carries `monitor_id`, `monitor_name`, `endpoint` for structured logging throughout the retry chain.

**Task 3.3: Update all `invokeClaudeCode()` call sites to pass context**

File: `docker/claude-agent/agent-server.js`

Search for all calls to `invokeClaudeCode(prompt)` and add context:
- `/analyze` handler (~line 1869): `invokeClaudeCode(prompt, WORK_DIR, { monitor_id: monitorId, monitor_name: monitorName, endpoint: '/analyze' })`
- `/watchdog` handler: same pattern with endpoint `/watchdog`
- `processGitHubIssue()`: context with `{ issue_number, repo_name, endpoint: '/github-issue' }`

---

### Phase 4: Failure Alerting -- Sidecar Side (alerting-engineer)

**Task 4.1: Implement `createFailureEvent()` in the sidecar**

File: `docker/claude-agent/agent-server.js`

Add function to create a Datadog event when all retries are exhausted. Uses the existing `DD_API_KEY`/`DD_APP_KEY` constants and the `httpRequest` utility (note: `httpRequest` uses `http` module; for `api.ddog-gov.com` we need `https` -- either extend `httpRequest` to support https or use `https` module directly).

```javascript
async function createFailureEvent(context, error) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        structuredLog('warn', 'failure_event_skip', { reason: 'no DD keys' });
        return null;
    }

    const eventPayload = {
        title: `[RCA Pipeline Failure] ${context.monitor_name || 'Unknown Monitor'} (ID: ${context.monitor_id || 'N/A'})`,
        text: [
            `%%% `,
            `### RCA Pipeline Failure`,
            ``,
            `**Monitor:** ${context.monitor_name || 'Unknown'} (ID: ${context.monitor_id || 'N/A'})`,
            `**Endpoint:** ${context.endpoint || 'N/A'}`,
            `**Error Type:** ${error.errorType || 'unknown'}`,
            `**Error:** ${error.message}`,
            `**Retries Exhausted:** ${error.retriesExhausted || false}`,
            `**Auth Method:** ${context.auth_method || 'N/A'}`,
            `**Timestamp:** ${new Date().toISOString()}`,
            ``,
            `The RCA analysis pipeline failed after exhausting all retries. Manual investigation required.`,
            ` %%%`
        ].join('\n'),
        alert_type: 'error',
        priority: 'normal',
        tags: [
            'service:claude-agent-sidecar',
            `error_type:${error.errorType || 'unknown'}`,
            `monitor_id:${context.monitor_id || 'unknown'}`,
            'source:rca_pipeline',
            'env:production'
        ],
        source_type_name: 'custom'
    };

    const url = `${DD_API_URL}/api/v1/events`;
    // Must use https for Datadog API
    return new Promise((resolve, reject) => {
        const body = JSON.stringify(eventPayload);
        const urlObj = new URL(url);
        const options = {
            hostname: urlObj.hostname,
            port: 443,
            path: urlObj.pathname,
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'DD-API-KEY': DD_API_KEY,
                'DD-APPLICATION-KEY': DD_APP_KEY,
                'Content-Length': Buffer.byteLength(body)
            }
        };

        const req = https.request(options, (res) => {
            let data = '';
            res.on('data', chunk => data += chunk);
            res.on('end', () => {
                if (res.statusCode >= 200 && res.statusCode < 300) {
                    structuredLog('info', 'failure_event_created', {
                        ...context,
                        dd_status: res.statusCode
                    });
                    resolve(JSON.parse(data));
                } else {
                    structuredLog('error', 'failure_event_failed', {
                        ...context,
                        dd_status: res.statusCode,
                        dd_response: data
                    });
                    resolve(null); // Don't fail the main flow
                }
            });
        });
        req.on('error', (e) => {
            structuredLog('error', 'failure_event_network_error', {
                ...context,
                error_message: e.message
            });
            resolve(null); // Don't fail the main flow
        });
        req.write(body);
        req.end();
    });
}
```

**CRITICAL:** Note that the existing `httpRequest()` helper on line 143 uses `http.request`, not `https.request`. The Datadog API at `api.ddog-gov.com` requires HTTPS. Either:
- (a) Modify `httpRequest()` to auto-select `http`/`https` based on URL protocol (preferred -- benefits all callers), or
- (b) Use `https.request` directly in `createFailureEvent()` as shown above.

Decision: Do option (a) as a prerequisite sub-task. Modify `httpRequest()` to check `urlObj.protocol === 'https:'` and use the appropriate module. This is a small change but high-impact since it unblocks Datadog API calls from the sidecar for both failure events AND any future direct API interactions.

**Task 4.2: Implement `createFailureNotebook()`**

File: `docker/claude-agent/agent-server.js`

A minimal version of the existing `createDatadogNotebook()` (which starts at line 172) but for failures. Creates a notebook with:
- Title: `[FAILED RCA] Monitor: {name} - {timestamp}`
- Cell 1 (markdown): Alert details (monitor ID, name, status, hostname, service)
- Cell 2 (markdown): Error details (error type, message, retry count)
- Cell 3 (markdown): Suggested manual investigation steps based on error type

```javascript
async function createFailureNotebook(context, error, originalPayload) {
    if (!DD_API_KEY || !DD_APP_KEY) return null;

    const timestamp = new Date().toISOString();
    const monitorId = context.monitor_id;
    const monitorName = context.monitor_name || 'Unknown';

    const notebook = {
        data: {
            attributes: {
                name: `[FAILED RCA] ${monitorName} - ${timestamp}`,
                time: {
                    live_span: '1h'
                },
                cells: [
                    {
                        attributes: {
                            definition: {
                                type: 'markdown',
                                text: [
                                    `# RCA Pipeline Failure Report`,
                                    ``,
                                    `**Generated:** ${timestamp}`,
                                    `**Monitor:** ${monitorName} (ID: ${monitorId || 'N/A'})`,
                                    `**Alert Status:** ${originalPayload?.alert_status || originalPayload?.ALERT_STATE || 'N/A'}`,
                                    `**Hostname:** ${originalPayload?.hostname || 'N/A'}`,
                                    `**Service:** ${originalPayload?.service || 'N/A'}`,
                                    ``,
                                    `> This notebook was auto-generated because the RCA analysis pipeline failed.`,
                                    `> The alert arrived but could not be analyzed. Manual investigation required.`
                                ].join('\n')
                            }
                        }
                    },
                    {
                        attributes: {
                            definition: {
                                type: 'markdown',
                                text: [
                                    `## Failure Details`,
                                    ``,
                                    `| Field | Value |`,
                                    `|-------|-------|`,
                                    `| Error Type | \`${error.errorType || 'unknown'}\` |`,
                                    `| Error Message | ${error.message} |`,
                                    `| Retries Exhausted | ${error.retriesExhausted || false} |`,
                                    `| Auth Method | ${context.auth_method || 'N/A'} |`,
                                    `| Endpoint | ${context.endpoint || 'N/A'} |`
                                ].join('\n')
                            }
                        }
                    },
                    {
                        attributes: {
                            definition: {
                                type: 'markdown',
                                text: getManualInvestigationSteps(error.errorType)
                            }
                        }
                    }
                ]
            },
            type: 'notebooks'
        }
    };

    // POST to Datadog Notebooks API (same pattern as createFailureEvent, using https)
    const url = `${DD_API_URL}/api/v1/notebooks`;
    // ... (same https.request pattern as createFailureEvent)
}

function getManualInvestigationSteps(errorType) {
    const steps = {
        auth: [
            `## Manual Investigation: Authentication Failure`,
            ``,
            `1. SSH into the host running the Claude agent sidecar`,
            `2. Check credentials: \`cat ~/.claude/.credentials.json | jq .claudeAiOauth.expiresAt\``,
            `3. If expired, run: \`claude login\` to re-authenticate`,
            `4. Restart the sidecar: \`kubectl rollout restart deployment/claude-agent\``,
            `5. Verify: \`curl -s http://localhost:9000/health | jq .auth\``
        ],
        network: [
            `## Manual Investigation: Network Failure`,
            ``,
            `1. Check if Claude API is reachable: \`curl -s https://api.anthropic.com/v1/messages -w "%{http_code}"\``,
            `2. Check DNS resolution: \`nslookup api.anthropic.com\``,
            `3. Check pod networking: \`kubectl exec -it deploy/claude-agent -- wget -qO- http://api.anthropic.com\``,
            `4. Check Qdrant connectivity: \`curl -s $QDRANT_URL/collections\``,
            `5. Check Ollama connectivity: \`curl -s $OLLAMA_URL/api/tags\``
        ],
        rate_limit: [
            `## Manual Investigation: Rate Limit`,
            ``,
            `1. Check current Claude usage tier in credentials: \`cat ~/.claude/.credentials.json | jq .claudeAiOauth.rateLimitTier\``,
            `2. Review recent invocation frequency in logs`,
            `3. Consider increasing the semaphore limit or adding request spacing`,
            `4. Wait 5-10 minutes and retry manually: \`curl -X POST http://localhost:9000/analyze ...\``
        ],
        unknown: [
            `## Manual Investigation: Unknown Error`,
            ``,
            `1. Check sidecar logs: \`kubectl logs deploy/claude-agent --tail=100\``,
            `2. Check sidecar health: \`curl -s http://localhost:9000/health\``,
            `3. Check if Claude CLI is installed: \`kubectl exec -it deploy/claude-agent -- claude --version\``,
            `4. Check disk space and memory in the pod`
        ]
    };
    return (steps[errorType] || steps.unknown).join('\n');
}
```

**Task 4.3: Wire failure alerting into the `/analyze` and `/watchdog` error catch blocks**

File: `docker/claude-agent/agent-server.js`

In the `/analyze` handler catch block (currently lines 1895-1901):

```javascript
} catch (err) {
    const errorType = err.errorType || classifyError(err, err.stderr || '');
    const context = { monitor_id: monitorId, monitor_name: monitorName, endpoint: '/analyze', auth_method: getAuthMethod() };

    structuredLog('error', 'analyze_pipeline_failed', {
        ...context,
        error_type: errorType,
        error_message: err.message,
        retries_exhausted: err.retriesExhausted || false
    });

    // Create Datadog failure event
    const ddEvent = await createFailureEvent(context, err);

    // Create failure notebook (optional, only for auth and unknown errors)
    let failureNotebook = null;
    if (errorType === 'auth' || errorType === 'unknown') {
        failureNotebook = await createFailureNotebook(context, err, fullPayload);
    }

    // Send desktop notification about the failure
    await sendDesktopNotification({
        ALERT_STATE: 'RCA_FAILURE',
        ALERT_TITLE: `[RCA FAILED] ${monitorName}`,
        DETAILED_DESCRIPTION: `Error: ${err.message} (type: ${errorType})`,
        URGENCY: 'HIGH',
        APPLICATION_TEAM: fullPayload.APPLICATION_TEAM || ''
    });

    sendJson(res, 500, {
        error: err.message,
        error_type: errorType,
        retries_exhausted: err.retriesExhausted || false,
        failure_event: ddEvent ? { id: ddEvent.event?.id } : null,
        failure_notebook: failureNotebook ? { id: failureNotebook.id, url: failureNotebook.url } : null,
        timestamp: new Date().toISOString()
    });
}
```

Apply the same pattern to the `/watchdog` handler's catch block.

**Task 4.4: Send desktop notification with failure-specific payload**

The `sendDesktopNotification()` function already exists and works. The modification here is to send a specially formatted notification when the pipeline fails, using `ALERT_STATE: 'RCA_FAILURE'` as a sentinel value so the desktop notification server can render it differently (e.g., red background, different sound).

This is handled inline in Task 4.3 above -- no separate function needed.

---

### Phase 5: Failure Alerting -- Go Server Side (alerting-engineer)

**Task 5.1: Create `FailureAlerter` in Go**

File: `mkii_ddog_server/services/agents/failure_alerter.go` (NEW FILE)

This is the Go-side supplementary alerter. It uses the existing `requests.Post` helper (from `cmd/utils/requests/`) to create Datadog events when the agent orchestrator detects a failure.

```go
package agents

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
)

type FailureAlerter struct {
    enabled bool
}

type DatadogEvent struct {
    Title          string   `json:"title"`
    Text           string   `json:"text"`
    AlertType      string   `json:"alert_type"`
    Priority       string   `json:"priority"`
    Tags           []string `json:"tags"`
    SourceTypeName string   `json:"source_type_name"`
}

func NewFailureAlerter() *FailureAlerter {
    return &FailureAlerter{enabled: true}
}

func (fa *FailureAlerter) ReportFailure(ctx context.Context, result *AnalysisResult, err error) {
    if !fa.enabled {
        return
    }

    title := fmt.Sprintf("[Go Orchestrator] Agent analysis failed: %s (Monitor %d)",
        result.MonitorName, result.MonitorID)

    text := fmt.Sprintf("Agent role: %s\nError: %s\nDuration: %v\nIterations: %d",
        result.AgentRole, result.Error, result.Duration, result.Iterations)

    event := DatadogEvent{
        Title:          title,
        Text:           text,
        AlertType:      "error",
        Priority:       "normal",
        Tags:           []string{
            "service:rayne",
            "source:agent_orchestrator",
            fmt.Sprintf("monitor_id:%d", result.MonitorID),
            fmt.Sprintf("agent_role:%s", result.AgentRole),
        },
        SourceTypeName: "custom",
    }

    // Use a background context since the original may be cancelled
    bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Create a minimal http.ResponseWriter and Request for the requests helper
    // OR: use a direct HTTP client call here since requests.Post expects handler context
    // Decision: Use direct http.Client since this is a background operation
    go fa.postEvent(bgCtx, event)
}
```

**Note:** The existing `requests.Post[T]` helper in `cmd/utils/requests/` is designed for handler context (takes `http.ResponseWriter` and `*http.Request`). For background operations like failure alerting, use a direct `http.Client` call with `DD-API-KEY` and `DD-APPLICATION-KEY` headers. Import the config to get the keys.

**Task 5.2: Wire `FailureAlerter` into `AgentOrchestrator.Analyze()`**

File: `mkii_ddog_server/services/agents/orchestrator.go`

Add a `failureAlerter` field to `AgentOrchestrator` struct. In `Analyze()`, after detecting failure (lines 130-136):

```go
if err != nil || (result != nil && !result.Success) {
    atomic.AddInt64(&o.totalErrors, 1)
    if o.failureAlerter != nil {
        o.failureAlerter.ReportFailure(ctx, result, err)
    }
}
```

**Task 5.3: Parse sidecar error response for `error_type`**

File: `mkii_ddog_server/services/agents/claude_agent.go`

Modify `claudeResponse` struct to include the new error fields:

```go
type claudeResponse struct {
    Success          bool   `json:"success"`
    MonitorID        int64  `json:"monitorId"`
    Analysis         string `json:"analysis"`
    Error            string `json:"error,omitempty"`
    ErrorType        string `json:"error_type,omitempty"`
    RetriesExhausted bool   `json:"retries_exhausted,omitempty"`
    Timestamp        string `json:"timestamp"`
    Notebook         *struct {
        URL string `json:"url"`
    } `json:"notebook,omitempty"`
    FailureEvent *struct {
        ID int64 `json:"id"`
    } `json:"failure_event,omitempty"`
    FailureNotebook *struct {
        ID  int64  `json:"id"`
        URL string `json:"url"`
    } `json:"failure_notebook,omitempty"`
}
```

In `invokeAnalysis()`, when `response.Error != ""`, include the error type in the returned error message:

```go
if response.Error != "" {
    errType := response.ErrorType
    if errType == "" {
        errType = "unknown"
    }
    return "", fmt.Errorf("agent error [%s]: %s (retries_exhausted: %v)",
        errType, response.Error, response.RetriesExhausted)
}
```

---

### Phase 6: httpRequest HTTPS Support (sidecar-resilience-engineer)

**Task 6.1: Modify `httpRequest()` to support HTTPS**

File: `docker/claude-agent/agent-server.js`, lines 143-170

This is a prerequisite for Tasks 4.1 and 4.2. The current `httpRequest()` uses only `http.request`. Modify it to auto-select the transport:

```javascript
function httpRequest(url, method, data = null, headers = {}) {
    return new Promise((resolve, reject) => {
        const urlObj = new URL(url);
        const transport = urlObj.protocol === 'https:' ? https : http;
        const options = {
            hostname: urlObj.hostname,
            port: urlObj.port || (urlObj.protocol === 'https:' ? 443 : 80),
            path: urlObj.pathname + urlObj.search,
            method: method,
            headers: { 'Content-Type': 'application/json', ...headers }
        };

        const req = transport.request(options, (res) => {
            let body = '';
            res.on('data', chunk => body += chunk);
            res.on('end', () => {
                try {
                    resolve({ status: res.statusCode, data: body ? JSON.parse(body) : null, headers: res.headers });
                } catch (e) {
                    resolve({ status: res.statusCode, data: body, headers: res.headers });
                }
            });
        });

        req.on('error', reject);
        if (data) req.write(JSON.stringify(data));
        req.end();
    });
}
```

Key changes:
1. Auto-select `http` vs `https` based on URL protocol
2. Default port 443 for HTTPS
3. Accept additional `headers` parameter (needed for DD-API-KEY, DD-APPLICATION-KEY)
4. Return `res.headers` in response object (needed for `Retry-After` parsing)

After this change, `createFailureEvent()` and `createFailureNotebook()` can use `httpRequest()` instead of raw `https.request`, simplifying those functions significantly.

---

### Phase 7: Integration Testing (integration-tester)

**Task 7.1: Test error classification**

Manual test script to verify `classifyError()` returns correct categories:

```bash
# Test from host by curling the sidecar directly

# 1. Simulate auth error: temporarily corrupt credentials.json
kubectl exec deploy/claude-agent -- \
  sh -c 'echo "{}" > ~/.claude/.credentials.json'

curl -X POST http://localhost:9000/analyze \
  -H "Content-Type: application/json" \
  -d '{"payload":{"monitor_id":999,"monitor_name":"test-auth-error","alert_status":"Alert"}}'

# Expect: 500 with error_type: "auth"
# Expect: Datadog event created
# Expect: Desktop notification with [RCA FAILED]

# 2. Restore credentials
kubectl cp ~/.claude/.credentials.json claude-agent-pod:/home/node/.claude/.credentials.json
```

**Task 7.2: Test retry behavior with structured log verification**

```bash
# Watch sidecar logs for structured JSON entries
kubectl logs -f deploy/claude-agent | jq 'select(.event | startswith("claude_invoke"))'

# Trigger an analysis and observe:
# - claude_invoke_start
# - claude_invoke_retry (if error occurs)
# - backoff_wait
# - analyze_pipeline_failed (if all retries fail)
# - failure_event_created
```

**Task 7.3: Test proactive token refresh**

```bash
# Modify expiresAt to be in the past (simulates near-expiry)
kubectl exec deploy/claude-agent -- \
  sh -c 'cat ~/.claude/.credentials.json | jq ".claudeAiOauth.expiresAt = $(date +%s)000" > /tmp/creds.json && mv /tmp/creds.json ~/.claude/.credentials.json'

# Trigger analysis - should see proactive_token_refresh in logs
curl -X POST http://localhost:9000/analyze \
  -H "Content-Type: application/json" \
  -d '{"payload":{"monitor_id":999,"monitor_name":"test-proactive-refresh","alert_status":"Alert"}}'
```

**Task 7.4: Verify Datadog event creation**

```bash
# After triggering a failure, check for the event in Datadog
# Via API:
curl -s "https://api.ddog-gov.com/api/v1/events?start=$(date -d '-1 hour' +%s)&end=$(date +%s)&tags=source:rca_pipeline" \
  -H "DD-API-KEY: $DD_API_KEY" \
  -H "DD-APPLICATION-KEY: $DD_APP_KEY" | jq '.events[] | {title, date_happened}'
```

**Task 7.5: Validate structured log format for Datadog pipeline**

Ensure all structured logs match this schema so a Datadog log pipeline can parse them:

```json
{
  "timestamp": "ISO8601",
  "level": "info|warn|error",
  "event": "string",
  "service": "claude-agent-sidecar",
  "monitor_id": "number (optional)",
  "monitor_name": "string (optional)",
  "error_type": "auth|network|rate_limit|unknown (optional)",
  "error_message": "string (optional)",
  "retry_count": "number (optional)",
  "duration_ms": "number (optional)"
}
```

Create a Datadog log pipeline (manual step) with:
- Grok parser for JSON logs
- Facets: `event`, `error_type`, `level`
- Monitor: "RCA Pipeline Failures" alerting on `event:analyze_pipeline_failed`

---

### Phase 8: Datadog Monitor Setup (alerting-engineer)

**Task 8.1: Create Datadog monitor for RCA pipeline failures**

This is a manual/Terraform step, not code. Document the monitor definition:

```
Monitor: RCA Pipeline Failure Alert
Type: Event Alert
Query: events("source:rca_pipeline error_type:*").rollup("count").by("error_type").last("5m") > 0
Priority: P2
Tags: service:claude-agent-sidecar, team:sre
Notification: @slack-sre-alerts @pagerduty-rca-failures
Message: |
  {{#is_alert}}
  The RCA analysis pipeline has failed {{value}} time(s) in the last 5 minutes.

  Error type: {{error_type}}
  Check the claude-agent-sidecar logs for details.

  Runbook: [RCA Pipeline Troubleshooting](link-to-runbook)
  {{/is_alert}}
```

**Task 8.2: Create Datadog log-based monitor as backup**

```
Monitor: RCA Structured Log Failures
Type: Log Alert
Query: logs("service:claude-agent-sidecar event:analyze_pipeline_failed").index("*").rollup("count").last("5m") > 0
```

This provides defense-in-depth: even if the event creation in Task 4.1 fails (e.g., DD_API_KEY missing), the structured logs will still reach Datadog via the log agent and this monitor will fire.

---

## Execution Order & Dependencies

```
Phase 6 (httpRequest HTTPS) ──┐
Phase 1 (Classification/Logging) ──┤
                                     ├── Phase 3 (Retry) ──┐
Phase 2 (Token Refresh) ────────────┘                      │
                                                            ├── Phase 4 (Sidecar Alerting)
                                                            │
Phase 5 (Go Alerting) ─────────────────────────────────────┘
                                                            │
Phase 7 (Testing) ──────────────────────────────────────────┘
Phase 8 (DD Monitors) ─────────────────────────────────────── (after Phase 7 validates)
```

**Critical path:** Phase 6 → Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 7

Phase 5 (Go side) can be done in parallel with Phases 2-4 since it touches different files.

## Risk Assessment

1. **OAuth refresh endpoint uncertainty:** The exact Anthropic OAuth2 token endpoint URL and request format need verification. The `claudeAiOauth` provider in credentials.json is specific to Claude CLI. If the endpoint requires a `client_id` or uses a non-standard grant type, Task 2.1 will need adjustment. **Mitigation:** Test the refresh flow manually first with `curl` before implementing.

2. **`httpRequest()` change scope:** Modifying `httpRequest()` to support HTTPS (Task 6.1) affects ALL existing callers (Qdrant, Ollama, notify server). Currently all callers use HTTP URLs, so the change is backward-compatible, but test all existing endpoints after modification.

3. **Retry storms under sustained failure:** If the Claude API is down for an extended period and multiple webhooks arrive, each will retry 4 times with backoff, potentially creating many concurrent retry chains. **Mitigation:** The Go orchestrator's semaphore (bounded concurrency) limits this. Document that the semaphore size should be kept low (2-3) during initial rollout.

4. **Credentials.json write conflicts:** `refreshOAuthToken()` writes to `credentials.json` while `invokeClaudeCodeCLI()` may be reading it. On Linux, `fs.writeFileSync` is atomic (write + rename), but add a simple file lock or write-then-rename pattern for safety.
