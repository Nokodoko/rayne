// Claude Agent HTTP Server
// Wraps Claude Code CLI invocations for RCA analysis and notebook generation
// Integrates with Qdrant vector DB and Ollama for embeddings

const http = require('http');
const https = require('https');
const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');

const PORT = process.env.PORT || 9000;
const WORK_DIR = '/app/work';
const ASSETS_DIR = '/app/assets';
const DD_LIB_DIR = '/app/dd_lib';
const QDRANT_URL = process.env.QDRANT_URL || 'http://qdrant-service:6333';
const OLLAMA_URL = process.env.OLLAMA_URL || 'http://ollama-service:11434';
const RCA_COLLECTION = 'rca_analyses';
const GO_PRINCIPLES_COLLECTION = 'go_principles';
const GONOTEBOOK_PATH = process.env.GONOTEBOOK_PATH || '/app/gonotebook';

// Datadog API configuration
// Construct API URL from DD_SITE (follows Datadog SDK convention)
const DD_API_KEY = process.env.DD_API_KEY;
const DD_APP_KEY = process.env.DD_APP_KEY;
const DD_SITE = process.env.DD_SITE || 'datadoghq.com';
const DD_API_URL = `https://api.${DD_SITE}`;
const DD_APP_URL = `https://app.${DD_SITE}`;  // For notebook hyperlinks

// Notebook lifecycle tracking: monitor_id -> { notebookId, monitorName, createdAt, status }
// Tracks which notebook was created for which monitor so recovery events can update them.
// Status transitions: Active -> Investigating -> Resolved
const notebookRegistry = new Map();

// Claude authentication configuration
const ANTHROPIC_API_KEY = process.env.ANTHROPIC_API_KEY;
const CLAUDE_AUTH_MODE = process.env.CLAUDE_AUTH_MODE || 'auto'; // 'token', 'apikey', or 'auto'
const CLAUDE_HOME = process.env.HOME || '/home/node';
const CLAUDE_CREDS_PATH = path.join(CLAUDE_HOME, '.claude', '.credentials.json');

// Notification server configuration - supports multiple servers via comma-separated NOTIFY_SERVER_URLS
const NOTIFY_SERVER_URLS = (() => {
    const urls = process.env.NOTIFY_SERVER_URLS;
    if (urls) {
        return urls.split(',').map(u => u.trim()).filter(u => u);
    }
    const singleUrl = process.env.NOTIFY_SERVER_URL || 'http://host.minikube.internal:9999';
    return [singleUrl];
})();

// Resolve the actual service name from a webhook payload.
// Custom Datadog webhook templates populate APPLICATION_TEAM with the real service
// name, while the standard `service` field often contains the monitor type
// (e.g., "http-check", "process-check") instead of the actual service.
// Priority: APPLICATION_TEAM > scope tag > service (if not a monitor type) > fallback
function resolveServiceName(payload) {
    // 1. APPLICATION_TEAM is the most reliable source from custom webhook templates
    const appTeam = payload.APPLICATION_TEAM || payload.application_team;
    if (appTeam && appTeam.trim()) {
        return appTeam.trim();
    }

    // 2. Check scope/tags for application_team tag
    const scope = payload.scope || '';
    const scopeMatch = scope.match(/application_team:([^,\s]+)/);
    if (scopeMatch) {
        return scopeMatch[1];
    }

    const tags = Array.isArray(payload.tags) ? payload.tags : [];
    for (const tag of tags) {
        const tagMatch = tag.match(/^application_team:(.+)$/);
        if (tagMatch) {
            return tagMatch[1];
        }
    }

    // 3. Use service only if it doesn't look like a monitor type
    const monitorTypePatterns = /^(http-check|process-check|tcp-check|dns-check|ssl-check|grpc-check|service-check|custom-check|metric alert|query alert|composite|synthetics|event-v2 alert|watchdog)$/i;
    const service = payload.service;
    if (service && service.trim() && !monitorTypePatterns.test(service.trim())) {
        return service.trim();
    }

    // 4. Fallback to raw service value
    return service || 'N/A';
}

// Derive a meaningful severity value from alert_status/priority fields.
// Ensures we never display 'N/A' for severity in notebooks.
// Mapping follows industry-standard incident priority conventions:
//   Alert/Triggered -> P2 (High)
//   Warn           -> P3 (Medium)
//   No Data        -> P4 (Low)
//   OK/Recovered   -> P5 (Info)
function deriveSeverity(payload) {
    // Explicit severity or priority from the payload takes precedence
    const explicit = payload.severity || payload.priority;
    if (explicit && explicit !== 'N/A' && explicit.trim()) {
        return explicit.trim();
    }

    // Map from URGENCY field (custom webhook templates)
    const urgency = (payload.URGENCY || payload.urgency || '').toLowerCase().trim();
    if (urgency === 'high' || urgency === 'critical') return 'P2 (High)';
    if (urgency === 'medium' || urgency === 'normal') return 'P3 (Medium)';
    if (urgency === 'low') return 'P4 (Low)';

    // Map from alert_status
    const alertStatus = (payload.alert_status || payload.alertStatus || '').toLowerCase().trim();
    const alertState = (payload.ALERT_STATE || '').toLowerCase().trim();

    if (alertStatus === 'alert' || alertState === 'triggered') return 'P2 (High)';
    if (alertStatus === 'warn') return 'P3 (Medium)';
    if (alertStatus === 'no data') return 'P4 (Low)';
    if (alertStatus === 'ok' || alertStatus === 'recovered') return 'P5 (Info)';

    // Final fallback: never return N/A
    return 'P3 (Medium)';
}

// Derive the environment value from tags or payload fields.
// Defaults to 'production' when no env tag is present rather than 'N/A'.
function deriveEnv(payload) {
    // Check explicit env field on payload
    if (payload.env && payload.env !== 'N/A' && payload.env.trim()) {
        return payload.env.trim();
    }

    // Check tags for env: tag
    const tags = payload.tags || [];
    const tagStr = Array.isArray(tags) ? tags.join(',') : (tags || '');
    const tagMatch = tagStr.match(/env:([^,\s]+)/);
    if (tagMatch) {
        return tagMatch[1];
    }

    // Check scope for env: tag
    const scope = payload.scope || '';
    const scopeMatch = scope.match(/env:([^,\s]+)/);
    if (scopeMatch) {
        return scopeMatch[1];
    }

    // Default to 'production' rather than 'N/A'
    return 'production';
}

// Send desktop notification via notify-server (same as webhook receive endpoint)
// Sends to all configured servers
async function sendDesktopNotification(payload) {
    const notifyPayload = {
        ALERT_STATE: payload.ALERT_STATE || payload.alert_status || '',
        ALERT_TITLE: payload.ALERT_TITLE || payload.monitor_name || '',
        APPLICATION_LONGNAME: payload.APPLICATION_LONGNAME || '',
        APPLICATION_TEAM: payload.APPLICATION_TEAM || payload.application_team || '',
        DETAILED_DESCRIPTION: payload.DETAILED_DESCRIPTION || '',
        IMPACT: payload.IMPACT || '',
        METRIC: payload.METRIC || '',
        SUPPORT_GROUP: payload.SUPPORT_GROUP || '',
        THRESHOLD: payload.THRESHOLD || '',
        VALUE: payload.VALUE || '',
        URGENCY: payload.URGENCY || ''
    };

    // Send to all configured servers
    const results = await Promise.allSettled(
        NOTIFY_SERVER_URLS.map(async (serverUrl) => {
            try {
                const response = await httpRequest(serverUrl, 'POST', notifyPayload);
                if (response.status === 200) {
                    console.log(`[Notify] Sent to ${serverUrl}: ${notifyPayload.ALERT_TITLE}`);
                } else {
                    console.log(`[Notify] ${serverUrl} returned status ${response.status}`);
                }
                return { serverUrl, success: response.status === 200 };
            } catch (err) {
                console.log(`[Notify] Failed to send to ${serverUrl}: ${err.message}`);
                return { serverUrl, success: false, error: err.message };
            }
        })
    );

    const successCount = results.filter(r => r.status === 'fulfilled' && r.value.success).length;
    console.log(`[Notify] Sent to ${successCount}/${NOTIFY_SERVER_URLS.length} servers`);
}

// Check available auth methods
function getAuthMethod() {
    if (CLAUDE_AUTH_MODE === 'apikey' && ANTHROPIC_API_KEY && ANTHROPIC_API_KEY !== 'placeholder-key') {
        return 'apikey';
    }
    if (CLAUDE_AUTH_MODE === 'token' && fs.existsSync(CLAUDE_CREDS_PATH)) {
        try {
            const creds = JSON.parse(fs.readFileSync(CLAUDE_CREDS_PATH, 'utf8'));
            // Check if credentials.json has actual content (not empty object)
            if (Object.keys(creds).length > 0) {
                return 'token';
            }
        } catch (e) {
            console.log(`[Auth] Failed to parse credentials.json: ${e.message}`);
        }
    }
    if (CLAUDE_AUTH_MODE === 'auto') {
        // Try token first
        if (fs.existsSync(CLAUDE_CREDS_PATH)) {
            try {
                const creds = JSON.parse(fs.readFileSync(CLAUDE_CREDS_PATH, 'utf8'));
                if (Object.keys(creds).length > 0) {
                    return 'token';
                }
            } catch (e) {
                // Fall through to API key check
            }
        }
        // Then try API key
        if (ANTHROPIC_API_KEY && ANTHROPIC_API_KEY !== 'placeholder-key') {
            return 'apikey';
        }
    }
    return null;
}

// Refresh the OAuth access token using the refresh token from credentials.json
async function refreshOAuthToken() {
    structuredLog('info', 'token_refresh_start', {});
    const credsRaw = fs.readFileSync(CLAUDE_CREDS_PATH, 'utf8');
    const creds = JSON.parse(credsRaw);
    const oauth = creds.claudeAiOauth;
    if (!oauth || !oauth.refreshToken) {
        throw new Error('No refresh token available in credentials.json');
    }
    const tokenUrl = 'https://console.anthropic.com/v1/oauth/token';
    const body = JSON.stringify({
        grant_type: 'refresh_token',
        refresh_token: oauth.refreshToken,
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

// Check if the OAuth token is expiring within the next 5 minutes
function isTokenExpiringSoon() {
    try {
        const creds = JSON.parse(fs.readFileSync(CLAUDE_CREDS_PATH, 'utf8'));
        const expiresAt = creds.claudeAiOauth?.expiresAt;
        if (!expiresAt) return false;
        return Date.now() > (expiresAt - 5 * 60 * 1000);
    } catch {
        return false;
    }
}

// Structured JSON logging for machine-parseable error telemetry
function structuredLog(level, event, data = {}) {
    const entry = {
        timestamp: new Date().toISOString(),
        level,
        event,
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

// Classify an error into a retry-actionable category
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

// Parse JSON body from request
function parseBody(req) {
    return new Promise((resolve, reject) => {
        let body = '';
        req.on('data', chunk => body += chunk);
        req.on('end', () => {
            try {
                resolve(body ? JSON.parse(body) : {});
            } catch (e) {
                reject(new Error('Invalid JSON'));
            }
        });
        req.on('error', reject);
    });
}

// Send JSON response
function sendJson(res, statusCode, data) {
    res.writeHead(statusCode, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(data));
}

// Make HTTP request helper - supports both HTTP and HTTPS
function httpRequest(url, method, data = null, extraHeaders = {}) {
    return new Promise((resolve, reject) => {
        const urlObj = new URL(url);
        const isHttps = urlObj.protocol === 'https:';
        const transport = isHttps ? https : http;
        const defaultPort = isHttps ? 443 : 80;
        const options = {
            hostname: urlObj.hostname,
            port: urlObj.port || defaultPort,
            path: urlObj.pathname + urlObj.search,
            method: method,
            headers: { 'Content-Type': 'application/json', ...extraHeaders }
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

// Create Datadog Notebook with incident report and hyperlinks
async function createDatadogNotebook(payload, analysis, similarRCAs = [], datadogUrls = null) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        console.log('[Notebook] Skipping - DD_API_KEY or DD_APP_KEY not set');
        return null;
    }

    const monitorId = payload.monitor_id || payload.monitorId;
    const monitorName = payload.monitor_name || payload.monitorName;
    const alertStatus = payload.alert_status || payload.alertStatus;
    const hostname = payload.hostname || 'N/A';
    const service = resolveServiceName(payload);
    const scope = payload.scope || 'N/A';
    const tags = payload.tags || [];
    const applicationTeam = payload.APPLICATION_TEAM || payload.application_team || 'N/A';
    const timestamp = new Date().toISOString();

    // Extract env/region/severity from tags or payload fields
    const tagStr = Array.isArray(tags) ? tags.join(',') : (tags || '');
    const env = deriveEnv(payload);
    const region = payload.region || (tagStr.match(/region:([^,\s]+)/)?.[1]) || 'N/A';
    const severity = deriveSeverity(payload);

    // Build default URLs if not provided
    const ddBaseUrl = DD_APP_URL;
    const nowTs = Math.floor(Date.now() / 1000) * 1000;
    const thirtyMinAgo = nowTs - (30 * 60 * 1000);

    const urls = datadogUrls || {
        monitor: monitorId ? `${ddBaseUrl}/monitors/${monitorId}` : null,
        host: hostname !== 'N/A' ? `${ddBaseUrl}/infrastructure?host=${encodeURIComponent(hostname.split('.')[0])}` : null,
        hostDashboard: hostname !== 'N/A' ? `${ddBaseUrl}/dash/integration/system_overview?tpl_var_host=${encodeURIComponent(hostname.split('.')[0])}` : null,
        logsHost: hostname !== 'N/A' ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`host:${hostname.split('.')[0]}*`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
        logsService: service !== 'N/A' ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`service:${service}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
        logsErrors: `${ddBaseUrl}/logs?query=${encodeURIComponent(`status:error`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
        apmService: service !== 'N/A' ? `${ddBaseUrl}/apm/services/${service}/operations` : null,
        apmTraces: service !== 'N/A' ? `${ddBaseUrl}/apm/traces?query=${encodeURIComponent(`service:${service}`)}&start=${thirtyMinAgo}&end=${nowTs}` : null,
        apmErrors: service !== 'N/A' ? `${ddBaseUrl}/apm/traces?query=${encodeURIComponent(`service:${service} status:error`)}&start=${thirtyMinAgo}&end=${nowTs}` : null,
        events: `${ddBaseUrl}/event/explorer?query=${encodeURIComponent('sources:*')}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
        eventsHost: hostname !== 'N/A' ? `${ddBaseUrl}/event/explorer?query=${encodeURIComponent(`host:${hostname.split('.')[0]}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
        dbm: service !== 'N/A' ? `${ddBaseUrl}/databases?query=${encodeURIComponent(`service:${service}`)}` : null,
        dbmQueries: `${ddBaseUrl}/databases/queries`,
        metrics: hostname !== 'N/A' ? `${ddBaseUrl}/metric/explorer?exp_metric=system.cpu.user&exp_scope=${encodeURIComponent(`host:${hostname.split('.')[0]}`)}` : null,
    };

    // Build similar RCAs markdown section
    let similarRCAsMarkdown = '';
    if (similarRCAs.length > 0) {
        similarRCAsMarkdown = `# 🧠 Similar Past Incidents\n\n` +
            `> Powered by vector database similarity search\n\n` +
            `---\n\n`;
        similarRCAs.forEach((rca, i) => {
            const rcaMonitorId = rca.payload?.monitor_id;
            const rcaMonitorName = rca.payload?.monitor_name || 'Unknown';
            const rcaMonitorUrl = rcaMonitorId ? `${ddBaseUrl}/monitors/${rcaMonitorId}` : null;
            const rcaDate = rca.payload?.timestamp ? new Date(rca.payload.timestamp).toISOString().split('T')[0] : 'N/A';
            const rcaAnalysisSnippet = rca.payload?.analysis?.substring(0, 200) || 'No analysis available';
            const score = Math.round(rca.score * 100);
            const scoreEmoji = score >= 90 ? '🟢' : score >= 75 ? '🟡' : '🟠';

            similarRCAsMarkdown += `## ${String.fromCharCode(0x2460 + i)} ${scoreEmoji} ${score}% Match — Similar incident pattern\n\n`;
            similarRCAsMarkdown += `### ${rcaMonitorUrl ? `[🔗 ${rcaMonitorName}](${rcaMonitorUrl})` : rcaMonitorName}\n\n`;
            similarRCAsMarkdown += `| | Detail |\n`;
            similarRCAsMarkdown += `|---|--------|\n`;
            similarRCAsMarkdown += `| 📆 **Date** | ${rcaDate} |\n`;
            similarRCAsMarkdown += `| 🔍 **Root Cause** | ${rcaAnalysisSnippet}... |\n`;
            similarRCAsMarkdown += `| ✅ **Resolution** | ${rca.payload?.resolution || 'N/A'} |\n\n`;
            similarRCAsMarkdown += `> ⚠️ **Relevance:** Pattern matches indicate similar failure mode\n\n`;
        });
    }

    // Build quick links section
    let quickLinksMarkdown = '### 🚀 Quick Links\n\n';
    quickLinksMarkdown += '| Link | Description |\n';
    quickLinksMarkdown += '|------|-------------|\n';
    if (urls.monitor) quickLinksMarkdown += `| 🎯 [View Monitor](${urls.monitor}) | Monitor configuration and alert history |\n`;
    if (urls.events) quickLinksMarkdown += `| 📅 [Events](${urls.events}) | Deployment events and config changes in the alert window |\n`;
    if (urls.dbmQueries) quickLinksMarkdown += `| 🗃️ [DB Queries](${urls.dbmQueries}) | Database query performance and slow queries |\n`;
    if (urls.apmErrors) quickLinksMarkdown += `| 🔎 [APM Traces](${urls.apmErrors}) | Distributed traces with error spans |\n`;
    if (urls.logsHost || urls.logsService) quickLinksMarkdown += `| 📜 [Logs](${urls.logsHost || urls.logsService || urls.logsErrors}) | Error logs from the affected service |\n`;

    // Build header with hyperlinks in table
    const tagsFormatted = tags.map(t => `\`${t}\``).join(' · ') || 'N/A';
    const headerMarkdown = `# 🚨 Incident Report: ${monitorName}\n` +
        `### \`service:${service}\` | \`env:${env}\` | \`region:${region}\`\n\n` +
        `> ⏰ **Generated:** ${timestamp}  |  ⚠️ **Status: ACTIVE**  |  📊 **Severity: ${severity}**\n\n` +
        `---\n\n` +
        `| | Field | Value |\n` +
        `|---|-------|-------|\n` +
        `| 🎯 | **Monitor** | ${urls.monitor ? `[🔗 #${monitorId}](${urls.monitor})` : `#${monitorId}`} |\n` +
        `| 🟥 | **Alert Status** | **⚠️ ${alertStatus}** |\n` +
        `| 🖥️ | **Hostname** | \`${hostname}\` |\n` +
        `| 📦 | **Service** | \`${service}\` |\n` +
        `| 🔍 | **Scope** | \`${scope}\` |\n` +
        `| 👥 | **Application Team** | ${applicationTeam} |\n` +
        `| 🏷️ | **Tags** | ${tagsFormatted} |\n\n` +
        `---\n\n` +
        quickLinksMarkdown;

    // Truncate notebook name to fit within 80 char limit
    // Format: "[Incident Report] {name} - YYYY-MM-DD" = 18 + name + 13 = 31 + name
    const maxNameLen = 80 - 31; // 49 chars for monitor name
    const truncatedName = monitorName.length > maxNameLen
        ? monitorName.substring(0, maxNameLen - 3) + '...'
        : monitorName;

    const notebookData = {
        data: {
            type: "notebooks",
            attributes: {
                name: `[Incident Report] ${truncatedName} - ${timestamp.split('T')[0]}`,
                cells: [
                    // Header cell with hyperlinks
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: headerMarkdown
                            }
                        }
                    },
                    // Analysis cell (Claude generates the full markdown with title)
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: analysis
                            }
                        }
                    },
                    // CPU metrics timeseries (if hostname available)
                    ...(hostname !== 'N/A' ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "timeseries",
                                show_legend: true,
                                requests: [
                                    {
                                        q: `avg:system.cpu.user{host:${hostname.split('.')[0]}*}`,
                                        display_type: "line",
                                        style: {
                                            line_width: "normal",
                                            palette: "dog_classic",
                                            line_type: "solid"
                                        }
                                    }
                                ],
                                yaxis: { scale: "linear" },
                                title: "CPU Usage"
                            },
                            graph_size: "m",
                            time: null
                        }
                    }] : []),
                    // Log stream widget (if hostname or service available)
                    ...((hostname !== 'N/A' || service !== 'N/A') ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## 📋 Related Logs\n\n` +
                                    `${urls.logsHost ? `- [View Host Logs](${urls.logsHost})\n` : ''}` +
                                    `${urls.logsService ? `- [View Service Logs](${urls.logsService})\n` : ''}` +
                                    `- [View Error Logs](${urls.logsErrors})\n`
                            }
                        }
                    }] : []),
                    // APM section (if service available)
                    ...(service !== 'N/A' ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## 🔍 APM & Traces\n\n` +
                                    `${urls.apmService ? `- [Service Overview](${urls.apmService})\n` : ''}` +
                                    `${urls.apmTraces ? `- [View All Traces](${urls.apmTraces})\n` : ''}` +
                                    `${urls.apmErrors ? `- [Error Traces Only](${urls.apmErrors})\n` : ''}` +
                                    `${urls.dbm ? `- [Database Monitoring](${urls.dbm})\n` : ''}`
                            }
                        }
                    }] : []),
                    // Similar incidents cell
                    ...(similarRCAs.length > 0 ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: similarRCAsMarkdown
                            }
                        }
                    }] : []),
                    // Footer cell with hyperlinks
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `---\n\n` +
                                    `> 🤖 *This incident report was automatically generated by the webhook agent at ${timestamp}*\n\n` +
                                    `### ↩️ Actions\n\n` +
                                    `| Action | Link |\n` +
                                    `|--------|------|\n` +
                                    `${urls.monitor ? `| 🎯 View Monitor | [🔗 Monitor #${monitorId}](${urls.monitor}) |\n` : ''}` +
                                    `${urls.monitor ? `| ✏️ Edit Monitor | [🔗 Edit thresholds & config](${urls.monitor}/edit) |\n` : ''}` +
                                    `${urls.events ? `| 📅 Related Events | [🔗 Event Explorer](${urls.events}) |\n` : ''}` +
                                    `\n### 🧬 Vector DB Metadata\n\n` +
                                    `| Key | Value |\n` +
                                    `|-----|-------|\n` +
                                    `| matches | ${similarRCAs.length} |\n` +
                                    `| threshold | 70% |\n`
                            }
                        }
                    }
                ],
                time: {
                    live_span: "1h"
                },
                status: "published"
            }
        }
    };

    try {
        console.log(`[Notebook] Creating incident report notebook for monitor ${monitorId}`);

        const response = await new Promise((resolve, reject) => {
            const urlObj = new URL(`${DD_API_URL}/api/v1/notebooks`);
            const options = {
                hostname: urlObj.hostname,
                port: urlObj.port || 443,
                path: urlObj.pathname,
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'DD-API-KEY': DD_API_KEY,
                    'DD-APPLICATION-KEY': DD_APP_KEY
                }
            };

            const req = https.request(options, (res) => {
                let body = '';
                res.on('data', chunk => body += chunk);
                res.on('end', () => {
                    try {
                        resolve({ status: res.statusCode, data: JSON.parse(body) });
                    } catch (e) {
                        resolve({ status: res.statusCode, data: body });
                    }
                });
            });

            req.on('error', reject);
            req.write(JSON.stringify(notebookData));
            req.end();
        });

        if (response.status === 200 || response.status === 201) {
            const notebookId = response.data?.data?.id;
            const notebookUrl = `${DD_APP_URL}/notebook/${notebookId}`;
            console.log(`[Notebook] Created successfully: ${notebookUrl}`);

            // Register notebook in lifecycle tracker
            if (monitorId) {
                notebookRegistry.set(String(monitorId), {
                    notebookId,
                    monitorName,
                    createdAt: timestamp,
                    status: 'Active',
                    type: 'incident'
                });
                console.log(`[Notebook] Registered notebook ${notebookId} for monitor ${monitorId} (Active)`);
            }

            return { id: notebookId, url: notebookUrl };
        } else {
            console.error(`[Notebook] Failed to create: ${response.status}`, response.data);
            return null;
        }
    } catch (err) {
        console.error(`[Notebook] Error creating notebook: ${err.message}`);
        return null;
    }
}

// Create Datadog Notebook for Watchdog monitor triggered events
async function createWatchdogNotebook(payload, analysis, triggerTime, similarRCAs = [], datadogUrls = null) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        console.log('[Watchdog Notebook] Skipping - DD_API_KEY or DD_APP_KEY not set');
        return null;
    }

    const monitorId = payload.monitor_id || payload.monitorId;
    const monitorName = payload.monitor_name || payload.monitorName;
    const alertStatus = payload.alert_status || payload.alertStatus;
    const hostname = payload.hostname || 'N/A';
    const service = resolveServiceName(payload);
    const tags = payload.tags || [];
    const applicationTeam = payload.APPLICATION_TEAM || payload.application_team || 'N/A';
    const timestamp = new Date().toISOString();

    // Extract env/region/severity from tags or payload fields
    const tagStr = Array.isArray(tags) ? tags.join(',') : (tags || '');
    const env = deriveEnv(payload);
    const region = payload.region || (tagStr.match(/region:([^,\s]+)/)?.[1]) || 'N/A';
    const severity = deriveSeverity(payload);

    const ddBaseUrl = DD_APP_URL;
    const nowTs = Math.floor(Date.now() / 1000) * 1000;
    const thirtyMinAgo = nowTs - (30 * 60 * 1000);

    const urls = datadogUrls || {
        watchdog: `${ddBaseUrl}/watchdog`,
        monitor: monitorId ? `${ddBaseUrl}/monitors/${monitorId}` : null,
        host: hostname !== 'N/A' ? `${ddBaseUrl}/infrastructure?host=${encodeURIComponent(hostname.split('.')[0])}` : null,
        hostDashboard: hostname !== 'N/A' ? `${ddBaseUrl}/dash/integration/system_overview?tpl_var_host=${encodeURIComponent(hostname.split('.')[0])}` : null,
        logsHost: hostname !== 'N/A' ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`host:${hostname.split('.')[0]}*`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
        logsService: service !== 'N/A' ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`service:${service}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
        logsErrors: `${ddBaseUrl}/logs?query=${encodeURIComponent(`status:error`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
        apmService: service !== 'N/A' ? `${ddBaseUrl}/apm/services/${service}/operations` : null,
        apmTraces: service !== 'N/A' ? `${ddBaseUrl}/apm/traces?query=${encodeURIComponent(`service:${service}`)}&start=${thirtyMinAgo}&end=${nowTs}` : null,
        apmErrors: service !== 'N/A' ? `${ddBaseUrl}/apm/traces?query=${encodeURIComponent(`service:${service} status:error`)}&start=${thirtyMinAgo}&end=${nowTs}` : null,
        events: `${ddBaseUrl}/event/explorer?query=${encodeURIComponent('sources:*')}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
        dbm: service !== 'N/A' ? `${ddBaseUrl}/databases?query=${encodeURIComponent(`service:${service}`)}` : null,
        dbmQueries: `${ddBaseUrl}/databases/queries`,
        metrics: hostname !== 'N/A' ? `${ddBaseUrl}/metric/explorer?exp_metric=system.cpu.user&exp_scope=${encodeURIComponent(`host:${hostname.split('.')[0]}`)}` : null,
    };

    // Build quick links
    let quickLinksMarkdown = '### 🚀 Quick Links\n\n';
    quickLinksMarkdown += '| Link | Description |\n';
    quickLinksMarkdown += '|------|-------------|\n';
    if (urls.watchdog) quickLinksMarkdown += `| 🐕 [Watchdog Dashboard](${urls.watchdog}) | Anomaly detection dashboard |\n`;
    if (urls.monitor) quickLinksMarkdown += `| 🎯 [View Monitor](${urls.monitor}) | Monitor configuration and alert history |\n`;
    if (urls.events) quickLinksMarkdown += `| 📅 [Events](${urls.events}) | Watchdog events in alert window |\n`;
    if (urls.apmService) quickLinksMarkdown += `| 🔎 [APM Service](${urls.apmService}) | Service performance and traces |\n`;
    if (urls.logsHost || urls.logsService) quickLinksMarkdown += `| 📜 [Logs](${urls.logsHost || urls.logsService || urls.logsErrors}) | Error logs from the affected service |\n`;

    // Header
    const tagsFormatted = tags.map(t => `\`${t}\``).join(' · ') || 'N/A';
    const headerMarkdown = `# 🐕 Watchdog Anomaly Alert: ${monitorName}\n` +
        `### \`service:${service}\` | \`env:${env}\` | \`region:${region}\`\n\n` +
        `> ⏰ **Generated:** ${timestamp}  |  ⚠️ **Status: ACTIVE**  |  📊 **Severity: ${severity}**\n\n` +
        `---\n\n` +
        `| | Field | Value |\n` +
        `|---|-------|-------|\n` +
        `| 🎯 | **Monitor** | ${urls.monitor ? `[🔗 #${monitorId}](${urls.monitor})` : `#${monitorId}`} |\n` +
        `| 🟥 | **Alert Status** | **⚠️ ${alertStatus}** |\n` +
        `| ⏱️ | **Triggered At** | ${triggerTime} |\n` +
        `| 🖥️ | **Hostname** | \`${hostname}\` |\n` +
        `| 📦 | **Service** | \`${service}\` |\n` +
        `| 👥 | **Application Team** | ${applicationTeam} |\n` +
        `| 🏷️ | **Tags** | ${tagsFormatted} |\n\n` +
        `---\n\n` +
        quickLinksMarkdown;

    // Similar incidents section
    let similarRCAsMarkdown = '';
    if (similarRCAs.length > 0) {
        similarRCAsMarkdown = `# 🧠 Similar Past Incidents\n\n` +
            `> Powered by vector database similarity search\n\n` +
            `---\n\n`;
        similarRCAs.forEach((rca, i) => {
            const rcaMonitorId = rca.payload?.monitor_id;
            const rcaMonitorName = rca.payload?.monitor_name || 'Unknown';
            const rcaMonitorUrl = rcaMonitorId ? `${ddBaseUrl}/monitors/${rcaMonitorId}` : null;
            const rcaDate = rca.payload?.timestamp ? new Date(rca.payload.timestamp).toISOString().split('T')[0] : 'N/A';
            const rcaAnalysisSnippet = rca.payload?.analysis?.substring(0, 200) || 'No analysis available';
            const score = Math.round(rca.score * 100);
            const scoreEmoji = score >= 90 ? '🟢' : score >= 75 ? '🟡' : '🟠';

            similarRCAsMarkdown += `## ${String.fromCharCode(0x2460 + i)} ${scoreEmoji} ${score}% Match — Similar anomaly pattern\n\n`;
            similarRCAsMarkdown += `### ${rcaMonitorUrl ? `[🔗 ${rcaMonitorName}](${rcaMonitorUrl})` : rcaMonitorName}\n\n`;
            similarRCAsMarkdown += `| | Detail |\n`;
            similarRCAsMarkdown += `|---|--------|\n`;
            similarRCAsMarkdown += `| 📆 **Date** | ${rcaDate} |\n`;
            similarRCAsMarkdown += `| 🔍 **Root Cause** | ${rcaAnalysisSnippet}... |\n`;
            similarRCAsMarkdown += `| ✅ **Resolution** | ${rca.payload?.resolution || 'N/A'} |\n\n`;
            similarRCAsMarkdown += `> ⚠️ **Relevance:** Pattern matches indicate similar failure mode\n\n`;
        });
    }

    // Truncate name for notebook title
    const maxNameLen = 80 - 36; // "[Watchdog Alert] {name} - YYYY-MM-DD"
    const truncatedName = monitorName.length > maxNameLen
        ? monitorName.substring(0, maxNameLen - 3) + '...'
        : monitorName;

    const notebookData = {
        data: {
            type: "notebooks",
            attributes: {
                name: `[Watchdog Alert] ${truncatedName} - ${timestamp.split('T')[0]}`,
                cells: [
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: headerMarkdown
                            }
                        }
                    },
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: analysis
                            }
                        }
                    },
                    // CPU metrics widget if hostname available
                    ...(hostname !== 'N/A' ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "timeseries",
                                show_legend: true,
                                requests: [{
                                    q: `avg:system.cpu.user{host:${hostname.split('.')[0]}*}`,
                                    display_type: "line",
                                    style: { line_width: "normal", palette: "dog_classic", line_type: "solid" }
                                }],
                                yaxis: { scale: "linear" },
                                title: "CPU Usage"
                            },
                            graph_size: "m",
                            time: null
                        }
                    }] : []),
                    // Related logs cell (if hostname or service available)
                    ...((hostname !== 'N/A' || service !== 'N/A') ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## 📋 Related Logs\n\n` +
                                    `${urls.logsHost ? `- [View Host Logs](${urls.logsHost})\n` : ''}` +
                                    `${urls.logsService ? `- [View Service Logs](${urls.logsService})\n` : ''}` +
                                    `- [View Error Logs](${urls.logsErrors})\n`
                            }
                        }
                    }] : []),
                    // APM section (if service available)
                    ...(service !== 'N/A' ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## 🔍 APM & Traces\n\n` +
                                    `${urls.apmService ? `- [Service Overview](${urls.apmService})\n` : ''}` +
                                    `${urls.apmTraces ? `- [View All Traces](${urls.apmTraces})\n` : ''}` +
                                    `${urls.apmErrors ? `- [Error Traces Only](${urls.apmErrors})\n` : ''}` +
                                    `${urls.dbm ? `- [Database Monitoring](${urls.dbm})\n` : ''}`
                            }
                        }
                    }] : []),
                    // Similar incidents
                    ...(similarRCAs.length > 0 ? [{
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: similarRCAsMarkdown
                            }
                        }
                    }] : []),
                    // Footer
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `---\n\n` +
                                    `> 🤖 *This watchdog alert report was automatically generated by the webhook agent at ${timestamp}*\n\n` +
                                    `### ↩️ Actions\n\n` +
                                    `| Action | Link |\n` +
                                    `|--------|------|\n` +
                                    `${urls.watchdog ? `| 🐕 Watchdog Dashboard | [🔗 View Anomalies](${urls.watchdog}) |\n` : ''}` +
                                    `${urls.monitor ? `| 🎯 View Monitor | [🔗 Monitor #${monitorId}](${urls.monitor}) |\n` : ''}` +
                                    `${urls.monitor ? `| ✏️ Edit Monitor | [🔗 Edit thresholds & config](${urls.monitor}/edit) |\n` : ''}` +
                                    `${urls.events ? `| 📅 Related Events | [🔗 Event Explorer](${urls.events}) |\n` : ''}` +
                                    `\n### 🧬 Vector DB Metadata\n\n` +
                                    `| Key | Value |\n` +
                                    `|-----|-------|\n` +
                                    `| matches | ${similarRCAs.length} |\n` +
                                    `| threshold | 70% |\n`
                            }
                        }
                    }
                ],
                time: { live_span: "1h" },
                status: "published"
            }
        }
    };

    try {
        console.log(`[Watchdog Notebook] Creating notebook for monitor ${monitorId}`);

        const response = await new Promise((resolve, reject) => {
            const urlObj = new URL(`${DD_API_URL}/api/v1/notebooks`);
            const options = {
                hostname: urlObj.hostname,
                port: urlObj.port || 443,
                path: urlObj.pathname,
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'DD-API-KEY': DD_API_KEY,
                    'DD-APPLICATION-KEY': DD_APP_KEY
                }
            };

            const req = https.request(options, (res) => {
                let body = '';
                res.on('data', chunk => body += chunk);
                res.on('end', () => {
                    try {
                        resolve({ status: res.statusCode, data: JSON.parse(body) });
                    } catch (e) {
                        resolve({ status: res.statusCode, data: body });
                    }
                });
            });

            req.on('error', reject);
            req.write(JSON.stringify(notebookData));
            req.end();
        });

        if (response.status === 200 || response.status === 201) {
            const notebookId = response.data?.data?.id;
            const notebookUrl = `${DD_APP_URL}/notebook/${notebookId}`;
            console.log(`[Watchdog Notebook] Created: ${notebookUrl}`);

            // Register notebook in lifecycle tracker
            if (monitorId) {
                notebookRegistry.set(String(monitorId), {
                    notebookId,
                    monitorName,
                    createdAt: timestamp,
                    status: 'Active',
                    type: 'watchdog'
                });
                console.log(`[Watchdog Notebook] Registered notebook ${notebookId} for monitor ${monitorId} (Active)`);
            }

            return { id: notebookId, url: notebookUrl };
        } else {
            console.error(`[Watchdog Notebook] Failed: ${response.status}`, response.data);
            return null;
        }
    } catch (err) {
        console.error(`[Watchdog Notebook] Error: ${err.message}`);
        return null;
    }
}

// Fetch an existing Datadog Notebook by ID
async function getDatadogNotebook(notebookId) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        return null;
    }

    try {
        const urlObj = new URL(`${DD_API_URL}/api/v1/notebooks/${notebookId}`);
        const response = await new Promise((resolve, reject) => {
            const options = {
                hostname: urlObj.hostname,
                port: urlObj.port || 443,
                path: urlObj.pathname,
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                    'DD-API-KEY': DD_API_KEY,
                    'DD-APPLICATION-KEY': DD_APP_KEY
                }
            };

            const req = https.request(options, (res) => {
                let body = '';
                res.on('data', chunk => body += chunk);
                res.on('end', () => {
                    try {
                        resolve({ status: res.statusCode, data: JSON.parse(body) });
                    } catch (e) {
                        resolve({ status: res.statusCode, data: body });
                    }
                });
            });
            req.on('error', reject);
            req.end();
        });

        if (response.status === 200) {
            return response.data;
        }
        console.error(`[Notebook] Failed to fetch notebook ${notebookId}: ${response.status}`);
        return null;
    } catch (err) {
        console.error(`[Notebook] Error fetching notebook ${notebookId}: ${err.message}`);
        return null;
    }
}

// Update an existing Datadog Notebook via PUT API
// The Datadog Notebooks API uses PUT (not PATCH) for updates
async function updateDatadogNotebook(notebookId, notebookData) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        return null;
    }

    try {
        const response = await new Promise((resolve, reject) => {
            const urlObj = new URL(`${DD_API_URL}/api/v1/notebooks/${notebookId}`);
            const options = {
                hostname: urlObj.hostname,
                port: urlObj.port || 443,
                path: urlObj.pathname,
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'DD-API-KEY': DD_API_KEY,
                    'DD-APPLICATION-KEY': DD_APP_KEY
                }
            };

            const req = https.request(options, (res) => {
                let body = '';
                res.on('data', chunk => body += chunk);
                res.on('end', () => {
                    try {
                        resolve({ status: res.statusCode, data: JSON.parse(body) });
                    } catch (e) {
                        resolve({ status: res.statusCode, data: body });
                    }
                });
            });

            req.on('error', reject);
            req.write(JSON.stringify(notebookData));
            req.end();
        });

        if (response.status === 200) {
            console.log(`[Notebook] Updated notebook ${notebookId} successfully`);
            return response.data;
        }
        console.error(`[Notebook] Failed to update notebook ${notebookId}: ${response.status}`, response.data);
        return null;
    } catch (err) {
        console.error(`[Notebook] Error updating notebook ${notebookId}: ${err.message}`);
        return null;
    }
}

// Resolve a notebook for recovery: updates its title and header to reflect RESOLVED status
// Follows incident disposition lifecycle: Active -> Investigating -> Resolved
async function resolveNotebook(notebookId, monitorId, monitorName, recoveryTimestamp) {
    console.log(`[Recovery] Resolving notebook ${notebookId} for monitor ${monitorId}`);

    // Fetch the existing notebook
    const existing = await getDatadogNotebook(notebookId);
    if (!existing) {
        console.error(`[Recovery] Could not fetch notebook ${notebookId}`);
        return null;
    }

    const notebook = existing.data;
    const attrs = notebook?.attributes;
    if (!attrs) {
        console.error(`[Recovery] Notebook ${notebookId} has no attributes`);
        return null;
    }

    // Update the notebook title: replace [Incident Report] or [Watchdog Alert] with [RESOLVED]
    const oldName = attrs.name || '';
    let newName = oldName;
    if (oldName.includes('[Incident Report]')) {
        newName = oldName.replace('[Incident Report]', '[RESOLVED]');
    } else if (oldName.includes('[Watchdog Alert]')) {
        newName = oldName.replace('[Watchdog Alert]', '[RESOLVED]');
    } else if (!oldName.includes('[RESOLVED]')) {
        newName = `[RESOLVED] ${oldName}`;
    }

    // Update the header cell: replace "Status: ACTIVE" with "Status: RESOLVED"
    const cells = attrs.cells || [];
    const updatedCells = cells.map(cell => {
        const text = cell?.attributes?.definition?.text || '';
        if (text.includes('Status: ACTIVE') || text.includes('Status:**')) {
            const updatedText = text
                .replace(/Status: ACTIVE/g, 'Status: RESOLVED')
                .replace(/⚠️ \*\*Status: ACTIVE\*\*/g, '✅ **Status: RESOLVED**');
            return {
                ...cell,
                attributes: {
                    ...cell.attributes,
                    definition: {
                        ...cell.attributes.definition,
                        text: updatedText
                    }
                }
            };
        }
        return cell;
    });

    // Add a resolution cell at the end (before the footer)
    const resolutionCell = {
        type: "notebook_cells",
        attributes: {
            definition: {
                type: "markdown",
                text: `# ✅ Incident Resolved\n\n` +
                    `> **Recovery detected at:** ${recoveryTimestamp}\n\n` +
                    `| Field | Value |\n` +
                    `|-------|-------|\n` +
                    `| Monitor | ${monitorName} (ID: ${monitorId}) |\n` +
                    `| Resolution Time | ${recoveryTimestamp} |\n` +
                    `| Status | **RESOLVED** |\n\n` +
                    `---\n\n` +
                    `> 🤖 *This resolution was automatically recorded by the webhook agent recovery pipeline*\n`
            }
        }
    };

    // Insert resolution cell before the last cell (footer)
    if (updatedCells.length > 1) {
        updatedCells.splice(updatedCells.length - 1, 0, resolutionCell);
    } else {
        updatedCells.push(resolutionCell);
    }

    // Build the update payload
    const updateData = {
        data: {
            type: "notebooks",
            attributes: {
                name: newName,
                cells: updatedCells,
                time: attrs.time || { live_span: "1h" },
                status: attrs.status || "published"
            }
        }
    };

    const result = await updateDatadogNotebook(notebookId, updateData);
    if (result) {
        // Update the registry
        const entry = notebookRegistry.get(String(monitorId));
        if (entry) {
            entry.status = 'Resolved';
            entry.resolvedAt = recoveryTimestamp;
        }
        console.log(`[Recovery] Notebook ${notebookId} resolved successfully`);
    }

    return result;
}

// Generate embeddings using Ollama with Gemma
async function generateEmbeddings(text) {
    try {
        const response = await httpRequest(`${OLLAMA_URL}/api/embeddings`, 'POST', {
            model: 'gemma:2b',
            prompt: text
        });
        if (response.status === 200 && response.data.embedding) {
            return response.data.embedding;
        }
        console.error(`[Ollama] Failed to generate embeddings: ${JSON.stringify(response)}`);
        return null;
    } catch (err) {
        console.error(`[Ollama] Error: ${err.message}`);
        return null;
    }
}

// Initialize Qdrant collection if not exists
async function initQdrantCollection() {
    try {
        // Check if collection exists
        const check = await httpRequest(`${QDRANT_URL}/collections/${RCA_COLLECTION}`, 'GET');
        if (check.status === 200) {
            console.log(`[Qdrant] Collection ${RCA_COLLECTION} already exists`);
            return true;
        }

        // Create collection with 2048 dimensions (gemma:2b embedding size)
        const create = await httpRequest(`${QDRANT_URL}/collections/${RCA_COLLECTION}`, 'PUT', {
            vectors: {
                size: 2048,
                distance: 'Cosine'
            }
        });
        console.log(`[Qdrant] Created collection: ${JSON.stringify(create)}`);
        return create.status === 200;
    } catch (err) {
        console.error(`[Qdrant] Init error: ${err.message}`);
        return false;
    }
}

// Store RCA in Qdrant vector DB with full payload
async function storeRCA(monitorId, monitorName, analysis, embedding, fullPayload = null) {
    try {
        const point = {
            id: Date.now(),
            vector: embedding,
            payload: {
                monitor_id: monitorId,
                monitor_name: monitorName,
                analysis: analysis,
                full_payload: fullPayload,
                application_team: fullPayload?.APPLICATION_TEAM || fullPayload?.application_team || null,
                service: fullPayload?.service || fullPayload?.Service || null,
                hostname: fullPayload?.hostname || fullPayload?.Hostname || null,
                alert_status: fullPayload?.alert_status || fullPayload?.AlertStatus || null,
                timestamp: new Date().toISOString()
            }
        };

        const response = await httpRequest(`${QDRANT_URL}/collections/${RCA_COLLECTION}/points`, 'PUT', {
            points: [point]
        });
        console.log(`[Qdrant] Stored RCA for monitor ${monitorId}: ${response.status}`);
        return response.status === 200;
    } catch (err) {
        console.error(`[Qdrant] Store error: ${err.message}`);
        return false;
    }
}

// Search similar RCAs in Qdrant
async function searchSimilarRCAs(embedding, limit = 5) {
    try {
        const response = await httpRequest(`${QDRANT_URL}/collections/${RCA_COLLECTION}/points/search`, 'POST', {
            vector: embedding,
            limit: limit,
            with_payload: true
        });
        if (response.status === 200 && response.data.result) {
            return response.data.result;
        }
        return [];
    } catch (err) {
        console.error(`[Qdrant] Search error: ${err.message}`);
        return [];
    }
}

// ============================================================================
// GoNotebook RAG Integration
// Chunks, embeds, and indexes Ultimate Go Notebook training material for
// retrieval-augmented generation in the GitHub issue processing pipeline.
// ============================================================================

// Initialize go_principles Qdrant collection if not exists
// Returns { exists: boolean, pointCount: number }
async function initGoPrinciplesCollection() {
    try {
        const check = await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}`, 'GET');
        if (check.status === 200) {
            // Collection exists, get point count
            const countResp = await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}/points/count`, 'POST', {});
            const pointCount = countResp.data?.result?.count || 0;
            console.log(`[GoNotebook] Collection exists with ${pointCount} points`);
            return { exists: true, pointCount };
        }

        // Create collection with same config as RCA (2048-dim gemma:2b embeddings)
        const create = await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}`, 'PUT', {
            vectors: {
                size: 2048,
                distance: 'Cosine'
            }
        });
        console.log(`[GoNotebook] Created collection: ${create.status}`);
        return { exists: create.status === 200, pointCount: 0 };
    } catch (err) {
        console.error(`[GoNotebook] Init error: ${err.message}`);
        return { exists: false, pointCount: 0 };
    }
}

// Chunk a markdown file by heading boundaries
// Splits by ## headings, further splits large sections by ### sub-headings
function chunkMarkdownFile(filePath, category, topic) {
    try {
        const content = fs.readFileSync(filePath, 'utf8');
        const chunks = [];
        const relPath = filePath.replace(GONOTEBOOK_PATH + '/', '');

        // Split by ## headings
        const sections = content.split(/^(?=## )/m);

        for (const section of sections) {
            if (!section.trim()) continue;

            // Extract heading from section
            const headingMatch = section.match(/^(#{2,3})\s+(.+)/);
            const sectionHeading = headingMatch ? headingMatch[2].trim() : 'Introduction';

            if (section.length > 3000) {
                // Further split large sections by ### sub-headings
                const subSections = section.split(/^(?=### )/m);
                for (const sub of subSections) {
                    if (!sub.trim()) continue;
                    const subHeadingMatch = sub.match(/^(#{2,3})\s+(.+)/);
                    const subHeading = subHeadingMatch ? subHeadingMatch[2].trim() : sectionHeading;
                    chunks.push({
                        text: sub.trim(),
                        metadata: {
                            source: 'goNotebook',
                            category,
                            topic,
                            file_path: relPath,
                            section_heading: subHeading
                        }
                    });
                }
            } else {
                chunks.push({
                    text: section.trim(),
                    metadata: {
                        source: 'goNotebook',
                        category,
                        topic,
                        file_path: relPath,
                        section_heading: sectionHeading
                    }
                });
            }
        }

        return chunks;
    } catch (err) {
        console.error(`[GoNotebook] Error chunking markdown ${filePath}: ${err.message}`);
        return [];
    }
}

// Chunk a Go source file (entire file as one chunk)
function chunkGoFile(filePath, category, topic) {
    try {
        const content = fs.readFileSync(filePath, 'utf8');
        if (!content.trim()) return [];

        const relPath = filePath.replace(GONOTEBOOK_PATH + '/', '');
        // Extract directory name as sub-topic context
        const dirName = path.basename(path.dirname(filePath));
        const contextLine = `// Go training example: ${topic} - ${dirName}\n`;

        return [{
            text: contextLine + content,
            metadata: {
                source: 'goNotebook',
                category,
                topic,
                file_path: relPath,
                section_heading: dirName
            }
        }];
    } catch (err) {
        console.error(`[GoNotebook] Error chunking Go file ${filePath}: ${err.message}`);
        return [];
    }
}

// Recursively walk a directory tree
function walkDir(dir) {
    let results = [];
    try {
        const entries = fs.readdirSync(dir, { withFileTypes: true });
        for (const entry of entries) {
            const fullPath = path.join(dir, entry.name);
            if (entry.isDirectory()) {
                // Skip irrelevant directories
                if (['.git', 'vendor', 'grpcExample', '.DS_Store'].includes(entry.name)) continue;
                results = results.concat(walkDir(fullPath));
            } else if (entry.isFile()) {
                // Skip non-content files
                if (['.gitignore', '.DS_Store'].includes(entry.name)) continue;
                if (entry.name.endsWith('.pem') || entry.name.endsWith('.crt') || entry.name.endsWith('.key')) continue;
                if (entry.name.endsWith('.md') || entry.name.endsWith('.go')) {
                    results.push(fullPath);
                }
            }
        }
    } catch (err) {
        console.error(`[GoNotebook] Error walking ${dir}: ${err.message}`);
    }
    return results;
}

// Categorize a file based on its path within the goNotebook directory
function categorizeFile(filePath) {
    const relPath = filePath.replace(GONOTEBOOK_PATH + '/', '');

    // gotraining/topics/go/README.md - the design philosophy bible
    if (relPath === 'gotraining/topics/go/README.md') {
        return { category: 'philosophy', topic: 'design_philosophy' };
    }

    // gotraining/topics/go/<category>/...
    const gotrainingMatch = relPath.match(/^gotraining\/topics\/go\/(\w+)\//);
    if (gotrainingMatch) {
        const category = gotrainingMatch[1];
        // Extract topic from next directory level
        const topicMatch = relPath.match(/^gotraining\/topics\/go\/\w+\/(\w+)/);
        const topic = topicMatch ? topicMatch[1] : category;
        return { category, topic };
    }

    // Root README
    if (relPath === 'README.md') {
        return { category: 'overview', topic: 'overview' };
    }

    // ultimate_go_notebook/chap{NN}/...
    const chapMatch = relPath.match(/^ultimate_go_notebook\/chap(\d+)/);
    if (chapMatch) {
        const chap = parseInt(chapMatch[1]);
        const topicMatch = relPath.match(/^ultimate_go_notebook\/chap\d+\/([^/]+)/);
        const dirTopic = topicMatch ? topicMatch[1].replace(/^\d+[-_]?/, '') : 'general';

        const chapMap = {
            2: { category: 'language', topic: 'syntax' },
            3: { category: 'language', topic: 'data_semantics' },
            4: { category: 'language', topic: 'decoupling' },
            5: { category: 'design', topic: dirTopic },
            6: { category: 'concurrency', topic: dirTopic },
            7: { category: 'testing', topic: dirTopic },
            8: { category: 'testing', topic: 'benchmarks' },
            9: { category: 'profiling', topic: dirTopic }
        };

        return chapMap[chap] || { category: 'general', topic: dirTopic };
    }

    // Fallback
    return { category: 'general', topic: 'general' };
}

// Discover all files in the goNotebook directory with categorization
function discoverGoNotebookFiles() {
    if (!fs.existsSync(GONOTEBOOK_PATH)) {
        console.error(`[GoNotebook] Directory not found: ${GONOTEBOOK_PATH}`);
        return [];
    }

    const allFiles = walkDir(GONOTEBOOK_PATH);
    const discovered = [];

    for (const filePath of allFiles) {
        const fileType = filePath.endsWith('.md') ? 'md' : 'go';
        const { category, topic } = categorizeFile(filePath);
        discovered.push({ filePath, fileType, category, topic });
    }

    console.log(`[GoNotebook] Discovered ${discovered.length} files`);
    return discovered;
}

// Sleep helper for rate limiting
function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// Ingest all goNotebook files into Qdrant
// Chunks files, generates embeddings via Ollama, and upserts to Qdrant in batches
async function ingestGoNotebook() {
    const files = discoverGoNotebookFiles();
    if (files.length === 0) {
        console.log('[GoNotebook] No files to ingest');
        return { chunksIngested: 0, errors: 0 };
    }

    // Prioritize the design philosophy README (ingest first)
    files.sort((a, b) => {
        if (a.category === 'philosophy') return -1;
        if (b.category === 'philosophy') return 1;
        return 0;
    });

    // Chunk all files
    let allChunks = [];
    for (const file of files) {
        const chunks = file.fileType === 'md'
            ? chunkMarkdownFile(file.filePath, file.category, file.topic)
            : chunkGoFile(file.filePath, file.category, file.topic);
        allChunks = allChunks.concat(chunks);
    }

    console.log(`[GoNotebook] Total chunks to ingest: ${allChunks.length}`);

    let ingested = 0;
    let errors = 0;
    const batchSize = 10;

    for (let i = 0; i < allChunks.length; i += batchSize) {
        const batch = allChunks.slice(i, i + batchSize);
        const points = [];

        for (const chunk of batch) {
            try {
                const embedding = await generateEmbeddings(chunk.text.substring(0, 4000));
                if (!embedding) {
                    errors++;
                    continue;
                }

                points.push({
                    id: Date.now() + ingested + errors,
                    vector: embedding,
                    payload: {
                        text: chunk.text,
                        ...chunk.metadata,
                        timestamp: new Date().toISOString()
                    }
                });
                ingested++;
            } catch (err) {
                console.error(`[GoNotebook] Embedding error: ${err.message}`);
                errors++;
            }
        }

        if (points.length > 0) {
            try {
                await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}/points`, 'PUT', {
                    points
                });
            } catch (err) {
                console.error(`[GoNotebook] Qdrant upsert error: ${err.message}`);
                errors += points.length;
                ingested -= points.length;
            }
        }

        if (i + batchSize < allChunks.length) {
            console.log(`[GoNotebook] Ingested ${ingested}/${allChunks.length} chunks`);
            await sleep(500); // Rate limit for Ollama
        }
    }

    console.log(`[GoNotebook] Ingestion complete: ${ingested} chunks ingested, ${errors} errors`);
    return { chunksIngested: ingested, errors };
}

// Search go_principles collection for relevant Go design principles
async function searchGoPrinciples(queryText, limit = 5) {
    try {
        const embedding = await generateEmbeddings(queryText.substring(0, 2000));
        if (!embedding) {
            console.log('[GoNotebook] Failed to generate query embedding');
            return [];
        }

        // Check if any category keywords are present for filtered search
        const categoryKeywords = {
            concurrency: ['concurrency', 'goroutine', 'channel', 'mutex', 'sync', 'parallel', 'async'],
            design: ['design', 'pattern', 'architecture', 'composition', 'interface', 'decouple', 'decoupling'],
            testing: ['test', 'testing', 'benchmark', 'mock', 'assert'],
            language: ['syntax', 'pointer', 'value', 'semantics', 'struct', 'slice', 'map', 'type'],
            profiling: ['profile', 'profiling', 'pprof', 'trace', 'memory', 'cpu'],
            packages: ['package', 'module', 'dependency', 'import'],
            generics: ['generic', 'generics', 'type parameter', 'constraint']
        };

        const lowerQuery = queryText.toLowerCase();
        let filter = null;

        for (const [category, keywords] of Object.entries(categoryKeywords)) {
            if (keywords.some(kw => lowerQuery.includes(kw))) {
                filter = {
                    should: [
                        { key: 'category', match: { value: category } },
                        { key: 'category', match: { value: 'philosophy' } }
                    ]
                };
                break;
            }
        }

        const searchPayload = {
            vector: embedding,
            limit,
            with_payload: true
        };
        if (filter) {
            searchPayload.filter = filter;
        }

        const response = await httpRequest(
            `${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}/points/search`,
            'POST',
            searchPayload
        );

        if (response.status === 200 && response.data.result) {
            return response.data.result.map(r => ({
                text: r.payload.text,
                score: r.score,
                metadata: {
                    category: r.payload.category,
                    topic: r.payload.topic,
                    file_path: r.payload.file_path,
                    section_heading: r.payload.section_heading
                }
            }));
        }

        return [];
    } catch (err) {
        console.error(`[GoNotebook] Search error: ${err.message}`);
        return [];
    }
}

// Format retrieved Go principles into a markdown context block for prompt injection
function formatGoPrinciplesContext(results) {
    if (!results || results.length === 0) return '';

    let context = `\n## Go Design Principles (Retrieved from Ultimate Go Notebook)\n\n`;
    context += `The following Go design principles and patterns are relevant to this feature request.\n`;
    context += `Follow these guidelines when implementing:\n\n`;

    let totalChars = context.length;
    const maxChars = 4000;

    for (const r of results) {
        const section = `### ${r.metadata.topic || 'General'} (${r.metadata.category || 'general'})\n${r.text}\n\n---\n\n`;

        if (totalChars + section.length > maxChars) {
            // Truncate this section to fit
            const remaining = maxChars - totalChars - 50;
            if (remaining > 200) {
                context += `### ${r.metadata.topic || 'General'} (${r.metadata.category || 'general'})\n${r.text.substring(0, remaining)}...\n\n`;
            }
            break;
        }

        context += section;
        totalChars += section.length;
    }

    return context;
}

// Execute dd_lib Python tools
function executeDDLibTool(toolName, params = {}) {
    return new Promise((resolve, reject) => {
        const args = [
            path.join(DD_LIB_DIR, 'dd_lib_tools.py'),
            toolName,
            JSON.stringify(params)
        ];

        console.log(`[DDLib] Executing tool: ${toolName} with params: ${JSON.stringify(params)}`);

        const python = spawn('python3', args, {
            cwd: DD_LIB_DIR,
            env: {
                ...process.env,
                PYTHONPATH: DD_LIB_DIR
            },
            stdio: ['ignore', 'pipe', 'pipe']
        });

        let stdout = '';
        let stderr = '';

        python.stdout.on('data', data => stdout += data.toString());
        python.stderr.on('data', data => stderr += data.toString());

        python.on('close', code => {
            if (code === 0) {
                try {
                    resolve(JSON.parse(stdout));
                } catch (e) {
                    resolve({ result: stdout });
                }
            } else {
                reject(new Error(`dd_lib tool failed: ${stderr || stdout}`));
            }
        });

        python.on('error', err => reject(err));
    });
}

// Create new dd_lib function (auto-write mode)
function createDDLibFunction(moduleName, functionCode) {
    return new Promise((resolve, reject) => {
        const modulePath = path.join(DD_LIB_DIR, `${moduleName}.py`);
        const separator = '\n\n# Auto-generated function\n';

        try {
            // Append to existing module or create new one
            let existingContent = '';
            if (fs.existsSync(modulePath)) {
                existingContent = fs.readFileSync(modulePath, 'utf8');
            } else {
                existingContent = `#!/usr/bin/env python3\n"""Auto-generated dd_lib module: ${moduleName}"""\n\nimport os\nimport requests\nfrom headers import headers\n`;
            }

            const newContent = existingContent + separator + functionCode + '\n';
            fs.writeFileSync(modulePath, newContent);

            console.log(`[DDLib] Created/updated function in ${modulePath}`);
            resolve({ success: true, path: modulePath });
        } catch (err) {
            reject(new Error(`Failed to create function: ${err.message}`));
        }
    });
}

// Load incident report template
function loadIncidentTemplate(templateId) {
    const templatePath = path.join(ASSETS_DIR, `${templateId}.json`);
    try {
        if (fs.existsSync(templatePath)) {
            return JSON.parse(fs.readFileSync(templatePath, 'utf8'));
        }
    } catch (e) {
        console.error(`[Template] Failed to load ${templateId}: ${e.message}`);
    }
    return null;
}

// Parse Retry-After value from an error message (returns ms or null)
function parseRetryAfter(err) {
    const match = (err.message || '').match(/retry.after[:\s]*(\d+)/i);
    if (match) return parseInt(match[1], 10) * 1000;
    return null;
}

// Retry wrapper with exponential backoff, error classification, and token refresh
async function retryWithBackoff(fn, context = {}) {
    const RETRY_CONFIG = {
        auth:       { maxRetries: 2, baseDelay: 1000 },
        rate_limit: { maxRetries: 3, baseDelay: 60000 },
        network:    { maxRetries: 4, baseDelay: 1000 },
        unknown:    { maxRetries: 0, baseDelay: 0 }
    };
    let lastError;
    const maxPossibleRetries = Math.max(...Object.values(RETRY_CONFIG).map(c => c.maxRetries));
    for (let attempt = 0; attempt <= maxPossibleRetries; attempt++) {
        try {
            if (attempt === 0 && getAuthMethod() === 'token' && isTokenExpiringSoon()) {
                structuredLog('info', 'proactive_token_refresh', context);
                try {
                    await refreshOAuthToken();
                } catch (refreshErr) {
                    // Non-fatal: Claude CLI handles its own auth internally
                    structuredLog('warn', 'proactive_refresh_skipped', {
                        ...context,
                        error_message: refreshErr.message
                    });
                }
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
            if (attempt >= config.maxRetries) break;
            if (errorType === 'auth') {
                try {
                    await refreshOAuthToken();
                    structuredLog('info', 'token_refresh_after_auth_error', context);
                } catch (refreshErr) {
                    structuredLog('error', 'token_refresh_failed', {
                        ...context,
                        error_message: refreshErr.message
                    });
                    break;
                }
            } else if (errorType === 'rate_limit') {
                const retryAfter = parseRetryAfter(err) || config.baseDelay;
                structuredLog('info', 'rate_limit_wait', { ...context, wait_ms: retryAfter });
                await sleep(retryAfter);
                continue;
            }
            const delay = config.baseDelay * Math.pow(2, attempt) + Math.random() * 1000;
            structuredLog('info', 'backoff_wait', { ...context, wait_ms: Math.round(delay) });
            await sleep(delay);
        }
    }
    lastError.retriesExhausted = true;
    lastError.errorType = lastError.errorType || classifyError(lastError, lastError.stderr || '');
    throw lastError;
}

// Invoke Claude using either CLI (token) or SDK (API key) with retry and backoff
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
            const err = new Error('No valid Claude authentication. Set ANTHROPIC_API_KEY or provide credentials.json via claude login');
            err.errorType = 'auth';
            throw err;
        }
    }, context);
}

// Original CLI approach (uses OAuth token from credentials.json)
function invokeClaudeCodeCLI(prompt, workDir = WORK_DIR) {
    return new Promise((resolve, reject) => {
        const args = ['--print', prompt];

        console.log(`[Claude CLI] Invoking with prompt: ${prompt.substring(0, 100)}...`);

        const claude = spawn('claude', args, {
            cwd: workDir,
            env: {
                ...process.env,
                PYTHONPATH: DD_LIB_DIR
            },
            stdio: ['ignore', 'pipe', 'pipe']  // Ignore stdin
        });

        let stdout = '';
        let stderr = '';

        claude.stdout.on('data', data => {
            stdout += data.toString();
        });

        claude.stderr.on('data', data => {
            stderr += data.toString();
            console.error(`[Claude CLI stderr] ${data.toString()}`);
        });

        claude.on('close', code => {
            console.log(`[Claude CLI] Exited with code ${code}`);
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
            const wrappedErr = new Error(`Failed to spawn Claude CLI: ${err.message}`);
            wrappedErr.errorType = classifyError(err, '');
            reject(wrappedErr);
        });
    });
}

// SDK approach (uses API key)
async function invokeClaudeCodeSDK(prompt) {
    console.log(`[Claude SDK] Invoking with prompt: ${prompt.substring(0, 100)}...`);

    try {
        // Dynamic import of Anthropic SDK
        const Anthropic = require('@anthropic-ai/sdk');
        const anthropic = new Anthropic.default({ apiKey: ANTHROPIC_API_KEY });

        const message = await anthropic.messages.create({
            model: "claude-sonnet-4-20250514",
            max_tokens: 8192,
            messages: [{ role: "user", content: prompt }]
        });

        console.log(`[Claude SDK] Response received, ${message.content.length} content blocks`);

        // Extract text from response
        const textContent = message.content.find(c => c.type === 'text');
        if (textContent) {
            return textContent.text;
        }
        return JSON.stringify(message.content);
    } catch (err) {
        console.error(`[Claude SDK] Error: ${err.message}`);
        throw new Error(`Claude SDK error: ${err.message}`);
    }
}

// Load template from assets
function loadTemplate(templateName) {
    const templatePath = path.join(ASSETS_DIR, templateName);
    try {
        return fs.readFileSync(templatePath, 'utf8');
    } catch (e) {
        console.error(`[Template] Failed to load ${templateName}: ${e.message}`);
        return null;
    }
}

// Process a GitHub issue using Claude Code CLI
async function processGitHubIssue(issueData) {
    const { issue_number, issue_title, issue_body, repo_name, sender_login, past_issues } = issueData;

    // Sanitize inputs - strip control characters, limit lengths
    const safeTitle = (issue_title || '').replace(/[\x00-\x1f\x7f]/g, '').substring(0, 200);
    const safeBody = (issue_body || '').replace(/[\x00-\x1f\x7f]/g, ' ').substring(0, 10000);
    const safeRepo = (repo_name || '').replace(/[^a-zA-Z0-9_./-]/g, '');

    console.log(`[GitHub] Processing issue #${issue_number}: ${safeTitle}`);

    // Build past issues context
    let pastIssuesContext = '';
    if (past_issues && past_issues.length > 0) {
        pastIssuesContext = `\n## Previously Processed Issues\n\nThe following issues have already been processed by the agent. Cross-reference these to detect duplicates:\n\n`;
        past_issues.forEach(pi => {
            pastIssuesContext += `- **Issue #${pi.number}**: ${pi.title}`;
            if (pi.branch_name) pastIssuesContext += ` (branch: \`${pi.branch_name}\`)`;
            if (pi.summary) pastIssuesContext += `\n  Summary: ${pi.summary}`;
            pastIssuesContext += ` [status: ${pi.agent_status}]\n`;
        });
        pastIssuesContext += `\n`;
    }

    // Retrieve relevant Go design principles from goNotebook RAG
    let goPrinciplesContext = '';
    try {
        const queryText = `${safeTitle} ${safeBody}`.substring(0, 1000);
        const principles = await searchGoPrinciples(queryText, 5);
        if (principles.length > 0) {
            goPrinciplesContext = formatGoPrinciplesContext(principles);
            console.log(`[GitHub] Retrieved ${principles.length} Go principles for issue #${issue_number}`);
        }
    } catch (err) {
        console.error(`[GitHub] Failed to retrieve Go principles: ${err.message}`);
        // Continue without principles - graceful degradation
    }

    const prompt = `You are implementing a feature request from GitHub issue #${issue_number} in the ${safeRepo} repository.

## Issue: ${safeTitle}

${safeBody}
${pastIssuesContext}
${goPrinciplesContext}
## CRITICAL: Duplicate Detection

Before implementing anything, carefully compare this issue against the Previously Processed Issues list above.

If this issue requests a feature that has ALREADY been implemented (same functionality, even if worded differently), you MUST:
1. Do NOT implement anything
2. Do NOT create a branch
3. Output a JSON summary with "duplicate": true indicating which issue already covers this

## Instructions (only if NOT a duplicate)

1. Read the CLAUDE.md and any agentic_instructions.md files in relevant directories to understand project patterns
2. **IMPORTANT**: Follow the Go design principles provided above (if present). Specifically:
   - Use value/pointer semantics consistently
   - Design interfaces based on behavior, not data
   - Handle errors as part of the main code path
   - Keep interfaces small (1-2 methods)
   - Write code that is readable by the average developer
3. Explore the existing codebase to understand architecture and conventions
4. Plan the implementation by identifying which files need to be created or modified
5. Implement the feature following existing patterns:
   - Go handlers return (int, any) for status code and response body
   - PostgreSQL uses $1, $2 placeholders
   - Route registration uses utils.Endpoint() or utils.EndpointWithPathParams()
   - Services follow the handler.go / storage.go / types.go pattern
6. Create a new git branch named "feature/issue-${issue_number}" from the current branch
7. Commit your changes with a descriptive message referencing issue #${issue_number}
8. Output a JSON summary in this exact format:

\`\`\`json
{
  "branch_name": "feature/issue-${issue_number}",
  "modified_files": ["list", "of", "modified", "files"],
  "new_types": ["list of new types/structs created"],
  "summary": "Brief description of what was implemented"
}
\`\`\`

## If this IS a duplicate, output ONLY this:

\`\`\`json
{
  "duplicate": true,
  "duplicate_of": <issue_number_that_already_covers_this>,
  "summary": "This feature was already implemented in issue #X which added <brief description>"
}
\`\`\`

IMPORTANT: Your final output MUST contain the JSON block above so results can be parsed.`;

    const result = await invokeClaudeCode(prompt, WORK_DIR, {
        issue_number: issue_number,
        repo_name: safeRepo,
        endpoint: '/github-issue'
    });

    // Parse JSON summary from Claude's output
    const jsonMatch = result.match(/```json\n([\s\S]*?)\n```/);
    let summary = {};
    if (jsonMatch) {
        try {
            summary = JSON.parse(jsonMatch[1]);
        } catch (e) {
            console.error(`[GitHub] Failed to parse JSON summary: ${e.message}`);
            summary = { summary: result.substring(0, 1000), raw: true };
        }
    } else {
        summary = { summary: result.substring(0, 1000), raw: true };
    }

    return summary;
}

// Build a markdown comment for the GitHub issue
function buildIssueComment(summary) {
    if (summary.raw) {
        return `## Agent Implementation Report\n\nThe agent processed this feature request.\n\n\`\`\`\n${summary.summary}\n\`\`\`\n\n---\n*Automated by Rayne Claude Agent*`;
    }

    let comment = `## Agent Implementation Report\n\n`;
    comment += `**Branch:** \`${summary.branch_name || 'unknown'}\`\n\n`;

    if (summary.modified_files && summary.modified_files.length > 0) {
        comment += `### Modified Files\n`;
        summary.modified_files.forEach(f => { comment += `- \`${f}\`\n`; });
        comment += `\n`;
    }

    if (summary.new_types && summary.new_types.length > 0) {
        comment += `### New Data Types\n`;
        summary.new_types.forEach(t => { comment += `- \`${t}\`\n`; });
        comment += `\n`;
    }

    if (summary.summary) {
        comment += `### Summary\n${summary.summary}\n\n`;
    }

    comment += `---\n*Automated by Rayne Claude Agent*`;
    return comment;
}

// Comment on a GitHub issue using gh CLI
async function commentOnGitHubIssue(repoName, issueNumber, body) {
    // Validate repo name format
    if (!/^[a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+$/.test(repoName)) {
        throw new Error(`Invalid repo name format: ${repoName}`);
    }
    if (!Number.isInteger(issueNumber) || issueNumber <= 0) {
        throw new Error(`Invalid issue number: ${issueNumber}`);
    }

    return new Promise((resolve, reject) => {
        const { spawn } = require('child_process');
        const gh = spawn('gh', ['issue', 'comment', String(issueNumber),
            '--repo', repoName, '--body', body], {
            env: { ...process.env },
            stdio: ['ignore', 'pipe', 'pipe']
        });

        const timeout = setTimeout(() => {
            gh.kill('SIGTERM');
            reject(new Error('gh comment timed out after 30s'));
        }, 30000);

        let stdout = '', stderr = '';
        gh.stdout.on('data', d => stdout += d);
        gh.stderr.on('data', d => stderr += d);
        gh.on('close', code => {
            clearTimeout(timeout);
            if (code === 0) {
                console.log(`[GitHub] Commented on ${repoName}#${issueNumber}`);
                resolve(stdout.trim());
            } else {
                console.error(`[GitHub] gh comment failed (code ${code}): ${stderr}`);
                reject(new Error(`gh exited ${code}: ${stderr}`));
            }
        });
        gh.on('error', err => {
            reject(new Error(`Failed to spawn gh: ${err.message}`));
        });
    });
}

// Close a GitHub issue with a comment using gh CLI
async function closeGitHubIssue(repoName, issueNumber, reason) {
    // Validate inputs
    if (!/^[a-zA-Z0-9_.-]+\/[a-zA-Z0-9_.-]+$/.test(repoName)) {
        throw new Error(`Invalid repo name format: ${repoName}`);
    }
    if (!Number.isInteger(issueNumber) || issueNumber <= 0) {
        throw new Error(`Invalid issue number: ${issueNumber}`);
    }

    // Comment first, then close
    await commentOnGitHubIssue(repoName, issueNumber, reason);

    return new Promise((resolve, reject) => {
        const { spawn } = require('child_process');
        const gh = spawn('gh', ['issue', 'close', String(issueNumber),
            '--repo', repoName, '--reason', 'not planned'], {
            env: { ...process.env },
            stdio: ['ignore', 'pipe', 'pipe']
        });

        const timeout = setTimeout(() => {
            gh.kill('SIGTERM');
            reject(new Error('gh close timed out after 30s'));
        }, 30000);

        let stdout = '', stderr = '';
        gh.stdout.on('data', d => stdout += d);
        gh.stderr.on('data', d => stderr += d);
        gh.on('close', code => {
            clearTimeout(timeout);
            if (code === 0) {
                console.log(`[GitHub] Closed ${repoName}#${issueNumber}`);
                resolve(stdout.trim());
            } else {
                console.error(`[GitHub] gh close failed (code ${code}): ${stderr}`);
                reject(new Error(`gh exited ${code}: ${stderr}`));
            }
        });
        gh.on('error', err => {
            reject(new Error(`Failed to spawn gh: ${err.message}`));
        });
    });
}

// ============================================================================
// Failure Alerting - Creates Datadog events and notebooks on pipeline failures
// ============================================================================

// Create a Datadog event to record an RCA pipeline failure
async function createFailureEvent(context, err) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        structuredLog('warn', 'failure_event_skip', { reason: 'no DD keys' });
        return null;
    }
    const errorType = err.errorType || classifyError(err, err.stderr || '');
    const title = `[RCA Pipeline Failure] ${context.monitor_name || 'Unknown Monitor'}`;
    const text = `## RCA Pipeline Failure\n\n` +
        `**Monitor:** ${context.monitor_name || 'N/A'} (ID: ${context.monitor_id || 'N/A'})\n` +
        `**Endpoint:** ${context.endpoint || 'N/A'}\n` +
        `**Error Type:** ${errorType}\n` +
        `**Error:** ${err.message}\n` +
        `**Retries Exhausted:** ${err.retriesExhausted || false}\n` +
        `**Timestamp:** ${new Date().toISOString()}\n`;

    const eventPayload = {
        title,
        text,
        priority: 'normal',
        tags: [
            'service:claude-agent-sidecar',
            `error_type:${errorType}`,
            `endpoint:${context.endpoint || 'unknown'}`,
            context.monitor_id ? `monitor_id:${context.monitor_id}` : null,
            'source:rca_pipeline'
        ].filter(Boolean),
        alert_type: 'error',
        source_type_name: 'custom'
    };

    try {
        const response = await httpRequest(`${DD_API_URL}/api/v1/events`, 'POST', eventPayload, {
            'DD-API-KEY': DD_API_KEY,
            'DD-APPLICATION-KEY': DD_APP_KEY
        });
        if (response.status === 200 || response.status === 202) {
            structuredLog('info', 'failure_event_created', {
                event_id: response.data?.event?.id,
                monitor_id: context.monitor_id
            });
            return response.data;
        }
        structuredLog('error', 'failure_event_api_error', {
            status: response.status,
            body: JSON.stringify(response.data).substring(0, 500)
        });
        return null;
    } catch (eventErr) {
        structuredLog('error', 'failure_event_error', { error_message: eventErr.message });
        return null;
    }
}

// Get manual investigation steps based on error type
function getManualInvestigationSteps(errorType) {
    const steps = {
        auth: [
            '1. Check if OAuth token in credentials.json has expired',
            '2. Run `claude login` to re-authenticate',
            '3. Verify ANTHROPIC_API_KEY environment variable if using API key mode',
            '4. Check Anthropic API status: https://status.anthropic.com'
        ],
        rate_limit: [
            '1. Check current API usage in Anthropic Console',
            '2. Review rate limit headers from recent responses',
            '3. Consider increasing retry delay or reducing concurrent analyses',
            '4. Check if other services are consuming the same API quota'
        ],
        network: [
            '1. Check DNS resolution: `nslookup console.anthropic.com`',
            '2. Test connectivity: `curl -v https://api.anthropic.com/v1/messages`',
            '3. Check firewall/proxy rules for outbound HTTPS',
            '4. Verify container network configuration and DNS settings'
        ],
        unknown: [
            '1. Check Claude agent sidecar logs for full stack trace',
            '2. Verify Claude CLI is installed and accessible in PATH',
            '3. Check container resource limits (memory/CPU)',
            '4. Review recent changes to agent-server.js or Dockerfile'
        ]
    };
    return steps[errorType] || steps.unknown;
}

// Create a minimal Datadog notebook documenting a pipeline failure
async function createFailureNotebook(context, err, fullPayload = {}) {
    if (!DD_API_KEY || !DD_APP_KEY) {
        return null;
    }
    const errorType = err.errorType || classifyError(err, err.stderr || '');
    const timestamp = new Date().toISOString();
    const monitorId = context.monitor_id || fullPayload.monitor_id || 'N/A';
    const monitorName = context.monitor_name || fullPayload.monitor_name || 'Unknown';
    const hostname = fullPayload.hostname || 'N/A';
    const service = fullPayload.service || 'N/A';
    const investigationSteps = getManualInvestigationSteps(errorType);

    const notebookData = {
        data: {
            type: "notebooks",
            attributes: {
                name: `[RCA Failure] ${monitorName.substring(0, 40)} - ${timestamp.split('T')[0]}`,
                cells: [
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `# RCA Pipeline Failure Report\n\n` +
                                    `**Generated:** ${timestamp}\n\n` +
                                    `---\n\n` +
                                    `| Field | Value |\n` +
                                    `|-------|-------|\n` +
                                    `| Monitor ID | ${monitorId} |\n` +
                                    `| Monitor Name | ${monitorName} |\n` +
                                    `| Hostname | ${hostname} |\n` +
                                    `| Service | ${service} |\n` +
                                    `| Alert Status | ${fullPayload.alert_status || fullPayload.alertStatus || 'N/A'} |\n`
                            }
                        }
                    },
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## Error Details\n\n` +
                                    `| Field | Value |\n` +
                                    `|-------|-------|\n` +
                                    `| Error Type | **${errorType}** |\n` +
                                    `| Error Message | ${err.message.substring(0, 500)} |\n` +
                                    `| Retries Exhausted | ${err.retriesExhausted || false} |\n` +
                                    `| Endpoint | ${context.endpoint || 'N/A'} |\n`
                            }
                        }
                    },
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## Manual Investigation Steps\n\n` +
                                    investigationSteps.join('\n') + '\n'
                            }
                        }
                    }
                ],
                time: { live_span: "1h" },
                status: "published"
            }
        }
    };

    try {
        const response = await httpRequest(`${DD_API_URL}/api/v1/notebooks`, 'POST', notebookData, {
            'DD-API-KEY': DD_API_KEY,
            'DD-APPLICATION-KEY': DD_APP_KEY
        });
        if (response.status === 200 || response.status === 201) {
            const notebookId = response.data?.data?.id;
            const notebookUrl = `${DD_APP_URL}/notebook/${notebookId}`;
            structuredLog('info', 'failure_notebook_created', { notebook_url: notebookUrl });
            return { id: notebookId, url: notebookUrl };
        }
        structuredLog('error', 'failure_notebook_api_error', { status: response.status });
        return null;
    } catch (nbErr) {
        structuredLog('error', 'failure_notebook_error', { error_message: nbErr.message });
        return null;
    }
}

// HTTP Server
const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, `http://localhost:${PORT}`);

    console.log(`[HTTP] ${req.method} ${url.pathname}`);

    // Health check
    if (url.pathname === '/health' && req.method === 'GET') {
        sendJson(res, 200, {
            status: 'healthy',
            timestamp: new Date().toISOString(),
            ddLibAvailable: fs.existsSync(DD_LIB_DIR),
            assetsAvailable: fs.existsSync(ASSETS_DIR)
        });
        return;
    }

    // RCA Analysis endpoint - accepts full webhook payload
    if (url.pathname === '/analyze' && req.method === 'POST') {
        let fullPayload = {};
        try {
            const body = await parseBody(req);
            const { payload, template_id, instructions } = body;

            // Support both new format (payload object) and legacy format
            fullPayload = payload || body;
            let monitorId = fullPayload.monitor_id || fullPayload.monitorId;
            let monitorName = fullPayload.monitor_name || fullPayload.monitorName;
            const alertStatus = fullPayload.alert_status || fullPayload.alertStatus;
            const scope = fullPayload.scope;
            const tags = fullPayload.tags;
            const hostname = fullPayload.hostname;
            const rawService = fullPayload.service;
            const service = resolveServiceName(fullPayload);
            const applicationTeam = fullPayload.APPLICATION_TEAM || fullPayload.application_team;

            // Write resolved service back to payload so downstream functions use it
            fullPayload.service = service;
            console.log(`[Analyze] Resolved service name: "${service}" (raw: "${rawService || 'N/A'}", APPLICATION_TEAM: "${applicationTeam || 'N/A'}")`);

            // Try to extract monitor ID from DETAILED_DESCRIPTION URL if not provided
            if (!monitorId && fullPayload.DETAILED_DESCRIPTION) {
                const match = fullPayload.DETAILED_DESCRIPTION.match(/monitors\/(\d+)/);
                if (match) {
                    monitorId = parseInt(match[1], 10);
                    console.log(`[Analyze] Extracted monitor ID from description: ${monitorId}`);
                }
            }

            // Fallback for test webhooks: use timestamp as unique ID
            if (!monitorId) {
                monitorId = Date.now();
                console.log(`[Analyze] Using generated ID for test webhook: ${monitorId}`);
            }

            // Fallback for monitor name from ALERT_TITLE
            if (!monitorName) {
                monitorName = fullPayload.ALERT_TITLE || fullPayload.alert_title || 'Unknown Monitor';
            }

            if (!monitorName) {
                sendJson(res, 400, { error: 'monitorName is required in payload' });
                return;
            }

            // Update fullPayload with resolved values (needed for createDatadogNotebook)
            fullPayload.monitor_id = monitorId;
            fullPayload.monitor_name = monitorName;

            // Send desktop notification (same as webhook receive endpoint)
            console.log(`[Analyze] Sending desktop notification for: ${monitorName}`);
            await sendDesktopNotification(fullPayload);

            // Load incident report template (JSON format)
            const incidentTemplate = template_id ? loadIncidentTemplate(template_id) : null;
            const markdownTemplate = loadTemplate('incident-report-cloned.md');

            // Generate embedding for the alert to find similar past RCAs
            const alertText = `Monitor: ${monitorName} Status: ${alertStatus} Scope: ${scope || 'N/A'} Host: ${hostname || 'N/A'} Service: ${service || 'N/A'}`;
            let similarRCAs = [];
            const alertEmbedding = await generateEmbeddings(alertText);

            if (alertEmbedding) {
                similarRCAs = await searchSimilarRCAs(alertEmbedding, 3);
                console.log(`[Analyze] Found ${similarRCAs.length} similar past RCAs`);
            }

            // Build context from similar RCAs
            let similarRCAContext = '';
            if (similarRCAs.length > 0) {
                similarRCAContext = `\n## Similar Past RCAs (for reference)\n`;
                similarRCAs.forEach((rca, i) => {
                    similarRCAContext += `\n### Past RCA ${i + 1} (similarity: ${(rca.score * 100).toFixed(1)}%)\n`;
                    similarRCAContext += `- Monitor: ${rca.payload.monitor_name}\n`;
                    similarRCAContext += `- Service: ${rca.payload.service || 'N/A'}\n`;
                    similarRCAContext += `- Analysis: ${rca.payload.analysis?.substring(0, 500) || 'N/A'}...\n`;
                });
            }

            // Pre-fetch Datadog data for comprehensive analysis
            console.log(`[Analyze] Pre-fetching Datadog data for analysis...`);

            // Datadog URL builder helper
            const ddBaseUrl = DD_APP_URL;
            const nowTs = Math.floor(Date.now() / 1000) * 1000;
            const thirtyMinAgo = nowTs - (30 * 60 * 1000);

            const buildDatadogUrls = (query, hostname, service, monitorId) => {
                const encodedQuery = encodeURIComponent(query || '');
                return {
                    logs: `${ddBaseUrl}/logs?query=${encodedQuery}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}&live=true`,
                    logsHost: hostname ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`host:${hostname.split('.')[0]}*`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
                    logsService: service ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`service:${service}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
                    logsErrors: `${ddBaseUrl}/logs?query=${encodeURIComponent(`status:error`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
                    apmService: service ? `${ddBaseUrl}/apm/services/${service}/operations` : null,
                    apmTraces: service ? `${ddBaseUrl}/apm/traces?query=${encodeURIComponent(`service:${service}`)}&start=${thirtyMinAgo}&end=${nowTs}` : null,
                    apmErrors: service ? `${ddBaseUrl}/apm/traces?query=${encodeURIComponent(`service:${service} status:error`)}&start=${thirtyMinAgo}&end=${nowTs}` : null,
                    host: hostname ? `${ddBaseUrl}/infrastructure?host=${encodeURIComponent(hostname.split('.')[0])}` : null,
                    hostDashboard: hostname ? `${ddBaseUrl}/dash/integration/system_overview?tpl_var_host=${encodeURIComponent(hostname.split('.')[0])}` : null,
                    monitor: monitorId ? `${ddBaseUrl}/monitors/${monitorId}` : null,
                    events: `${ddBaseUrl}/event/explorer?query=${encodeURIComponent('sources:*')}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
                    eventsHost: hostname ? `${ddBaseUrl}/event/explorer?query=${encodeURIComponent(`host:${hostname.split('.')[0]}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
                    dbm: service ? `${ddBaseUrl}/databases?query=${encodeURIComponent(`service:${service}`)}` : null,
                    dbmQueries: `${ddBaseUrl}/databases/queries`,
                    metrics: hostname ? `${ddBaseUrl}/metric/explorer?exp_metric=system.cpu.user&exp_scope=${encodeURIComponent(`host:${hostname.split('.')[0]}`)}` : null,
                };
            };

            const datadogUrls = buildDatadogUrls(null, hostname, service, monitorId);

            let logsData = null;
            let hostData = null;
            let eventsData = null;
            let monitorData = null;
            let logQuery = null;

            // Fetch logs related to the alert
            try {
                logQuery = hostname
                    ? `host:${hostname.split('.')[0]}* status:error OR status:warn`
                    : service
                        ? `service:${service} status:error OR status:warn`
                        : `status:error`;
                console.log(`[Analyze] Fetching logs with query: ${logQuery}`);
                logsData = await executeDDLibTool('search_logs', {
                    query: logQuery,
                    from_time: 'now-30m',
                    to_time: 'now',
                    limit: 20
                });
                // Update URLs with actual query used
                datadogUrls.logsQuery = `${ddBaseUrl}/logs?query=${encodeURIComponent(logQuery)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}&live=true`;
                console.log(`[Analyze] Fetched ${logsData?.count || 0} log entries`);
            } catch (err) {
                console.log(`[Analyze] Failed to fetch logs: ${err.message}`);
            }

            // Fetch host information if hostname is available
            if (hostname) {
                try {
                    console.log(`[Analyze] Fetching host info for: ${hostname}`);
                    hostData = await executeDDLibTool('get_host_info', { hostname: hostname.split('.')[0] });
                    console.log(`[Analyze] Host info fetched: ${hostData?.hostname || 'not found'}`);
                } catch (err) {
                    console.log(`[Analyze] Failed to fetch host info: ${err.message}`);
                }
            }

            // Fetch recent events
            try {
                console.log(`[Analyze] Fetching recent events...`);
                eventsData = await executeDDLibTool('get_events', {
                    from_time: Math.floor(Date.now() / 1000) - 1800, // Last 30 min
                    to_time: Math.floor(Date.now() / 1000)
                });
                console.log(`[Analyze] Fetched ${eventsData?.count || 0} events`);
            } catch (err) {
                console.log(`[Analyze] Failed to fetch events: ${err.message}`);
            }

            // Fetch monitor details
            if (monitorId) {
                try {
                    console.log(`[Analyze] Fetching monitor details for: ${monitorId}`);
                    monitorData = await executeDDLibTool('get_monitor_details', { monitor_id: monitorId });
                    console.log(`[Analyze] Monitor details fetched: ${monitorData?.name || 'not found'}`);
                } catch (err) {
                    console.log(`[Analyze] Failed to fetch monitor details: ${err.message}`);
                }
            }

            // Build Datadog context section with hyperlinks
            let datadogContext = '\n## Live Datadog Data\n';
            datadogContext += `\n**Quick Links:** `;
            datadogContext += datadogUrls.monitor ? `[Monitor](${datadogUrls.monitor}) | ` : '';
            datadogContext += datadogUrls.logsQuery ? `[Logs](${datadogUrls.logsQuery}) | ` : '';
            datadogContext += datadogUrls.apmService ? `[APM Service](${datadogUrls.apmService}) | ` : '';
            datadogContext += datadogUrls.host ? `[Host Infrastructure](${datadogUrls.host}) | ` : '';
            datadogContext += datadogUrls.events ? `[Events](${datadogUrls.events})` : '';
            datadogContext += `\n`;

            if (logsData && logsData.logs && logsData.logs.length > 0) {
                datadogContext += `\n### Recent Error/Warning Logs (last 30 min)\n`;
                datadogContext += `📋 [View all ${logsData.count} logs in Datadog](${datadogUrls.logsQuery})\n\n`;
                datadogContext += `Found ${logsData.count} relevant log entries:\n\n`;
                logsData.logs.slice(0, 10).forEach((log, i) => {
                    const logService = log.service || 'unknown';
                    const logHost = log.host || 'unknown';
                    const logServiceUrl = `${ddBaseUrl}/logs?query=${encodeURIComponent(`service:${logService}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`;
                    const logHostUrl = `${ddBaseUrl}/logs?query=${encodeURIComponent(`host:${logHost}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`;

                    datadogContext += `**${i + 1}. [${log.status || 'INFO'}] ${log.timestamp || 'N/A'}**\n`;
                    datadogContext += `- Service: [${logService}](${logServiceUrl})\n`;
                    datadogContext += `- Host: [${logHost}](${logHostUrl})\n`;
                    datadogContext += `- Message: ${(log.message || '').substring(0, 300)}${log.message?.length > 300 ? '...' : ''}\n`;
                    // Add trace link if trace_id is available
                    if (log.trace_id) {
                        datadogContext += `- 🔗 [View Trace](${ddBaseUrl}/apm/trace/${log.trace_id})\n`;
                    }
                    datadogContext += `\n`;
                });
            } else {
                datadogContext += `\n### Recent Logs\n`;
                datadogContext += `No error/warning logs found. [Search all logs](${datadogUrls.logsErrors})\n`;
            }

            if (hostData && !hostData.error) {
                datadogContext += `\n### Host Information: [${hostData.hostname || hostname}](${datadogUrls.host})\n`;
                datadogContext += `📊 [View Host Dashboard](${datadogUrls.hostDashboard}) | [View Metrics](${datadogUrls.metrics})\n\n`;
                datadogContext += `- Status: ${hostData.up ? '🟢 UP' : '🔴 DOWN'}\n`;
                datadogContext += `- Is Muted: ${hostData.is_muted || false}\n`;
                datadogContext += `- Apps: ${hostData.apps?.join(', ') || 'N/A'}\n`;
                datadogContext += `- Sources: ${hostData.sources?.join(', ') || 'N/A'}\n`;
                if (hostData.metrics) {
                    datadogContext += `- CPU: ${hostData.metrics.cpu || 'N/A'}%\n`;
                    datadogContext += `- Memory: ${hostData.metrics.memory || 'N/A'}%\n`;
                    datadogContext += `- Load: ${hostData.metrics.load || 'N/A'}\n`;
                }
            } else if (hostname) {
                datadogContext += `\n### Host: [${hostname}](${datadogUrls.host})\n`;
                datadogContext += `Host details not available. [View in Infrastructure](${datadogUrls.host})\n`;
            }

            if (service) {
                datadogContext += `\n### APM Service: [${service}](${datadogUrls.apmService})\n`;
                datadogContext += `🔍 [View Traces](${datadogUrls.apmTraces}) | [Error Traces](${datadogUrls.apmErrors})`;
                if (datadogUrls.dbm) {
                    datadogContext += ` | [Database Queries](${datadogUrls.dbm})`;
                }
                datadogContext += `\n`;
            }

            if (eventsData && eventsData.events && eventsData.events.length > 0) {
                datadogContext += `\n### Recent Events (last 30 min)\n`;
                datadogContext += `📅 [View all events in Event Explorer](${datadogUrls.events})\n\n`;
                eventsData.events.slice(0, 5).forEach((event, i) => {
                    const eventUrl = event.id ? `${ddBaseUrl}/event/event?id=${event.id}` : datadogUrls.events;
                    datadogContext += `${i + 1}. **[${event.alert_type || 'info'}]** [${event.title || 'N/A'}](${eventUrl})\n`;
                    datadogContext += `   - Source: ${event.source || 'N/A'}\n`;
                    if (event.host) {
                        const eventHostUrl = `${ddBaseUrl}/infrastructure?host=${encodeURIComponent(event.host)}`;
                        datadogContext += `   - Host: [${event.host}](${eventHostUrl})\n`;
                    }
                });
            } else {
                datadogContext += `\n### Recent Events\n`;
                datadogContext += `No recent events. [View Event Explorer](${datadogUrls.events})\n`;
            }

            if (monitorData && !monitorData.error) {
                datadogContext += `\n### Monitor Configuration: [${monitorData.name}](${datadogUrls.monitor})\n`;
                datadogContext += `- Type: ${monitorData.type}\n`;
                datadogContext += `- Query: \`${monitorData.query || 'N/A'}\`\n`;
                datadogContext += `- Created by: ${monitorData.creator || 'N/A'}\n`;
                datadogContext += `- 🔗 [Edit Monitor](${datadogUrls.monitor}/edit)\n`;
            } else if (monitorId) {
                datadogContext += `\n### Monitor: [View Monitor #${monitorId}](${datadogUrls.monitor})\n`;
            }

            // Add Database Monitoring section if service is database-related
            if (service && (service.includes('postgres') || service.includes('mysql') || service.includes('db') || service.includes('database'))) {
                datadogContext += `\n### Database Monitoring\n`;
                datadogContext += `🗄️ [View Database Queries](${datadogUrls.dbmQueries}) | [Service DBM](${datadogUrls.dbm})\n`;
            }

            // Build comprehensive prompt with full payload context AND live data
            const prompt = `You are an SRE analyzing a Datadog alert. You have been provided with LIVE data from Datadog including recent logs, host information, and events. Use this data to provide evidence-based root cause analysis.

${instructions || ''}

## Full Alert Payload
${JSON.stringify(fullPayload, null, 2)}

## Alert Summary
- Monitor: ${monitorName} (ID: ${monitorId})
- Status: ${alertStatus}
- Scope: ${scope || 'N/A'}
- Hostname: ${hostname || 'N/A'}
- Service: ${service || 'N/A'}
- Application Team: ${applicationTeam || 'N/A'}
- Tags: ${tags?.join(', ') || 'N/A'}

${datadogContext}

${similarRCAContext}

${incidentTemplate ? `## Output Template\n${JSON.stringify(incidentTemplate, null, 2)}` : ''}

## Analysis Output Format

Structure your analysis as Datadog Notebook markdown:

### Start with:
# 🔬 Root Cause Analysis
> **Assessment:** {one sentence summary of root cause}

### Then evidence sections numbered ① ② ③ ④:
Each evidence section should follow this pattern:
### ① 📜 Log Evidence — {brief label}
\`\`\`
{paste actual evidence from the data above}
\`\`\`
> 💡 {your insight interpreting this evidence}

Use these evidence types as applicable (skip if no data):
- 📜 Log Evidence
- 📊 Metric Evidence
- 📅 Event Evidence (deploys, config changes)
- 🔎 APM Trace Evidence

### Then include:
## 🎯 Confidence Level
| Level | Assessment | Reasoning |
|-------|-----------|-----------|
| 🟢/🟡/🟠 | High/Medium/Low | {your reasoning based on evidence quality} |

## 🔧 Immediate Actions
| Priority | Action | Rationale |
|----------|--------|-----------|
| 🔴 P1 | {most urgent action} | {why} |
| 🟡 P2 | {second action} | {why} |

## 🌊 Related Impact
| Affected | Type | Evidence |
|----------|------|----------|
| {service or host name} | {downstream/upstream/shared-resource} | {what evidence shows this} |

Use code blocks for actual log lines, metric values, and trace data. Use > 💡 for insights.
Cite specific data from the Datadog context above. Do NOT fabricate evidence.`;

            console.log(`[Analyze] Processing alert for monitor ${monitorId}: ${monitorName}`);
            console.log(`[Analyze] Full payload received with ${Object.keys(fullPayload).length} fields`);

            const result = await invokeClaudeCode(prompt, WORK_DIR, {
                monitor_id: monitorId,
                monitor_name: monitorName,
                endpoint: '/analyze'
            });

            // Store this RCA in vector DB with full payload for future reference
            if (alertEmbedding) {
                const stored = await storeRCA(monitorId, monitorName, result, alertEmbedding, fullPayload);
                console.log(`[Analyze] RCA stored in vector DB with full payload: ${stored}`);
            }

            // Create Datadog Notebook with incident report and hyperlinks
            const notebook = await createDatadogNotebook(fullPayload, result, similarRCAs, datadogUrls);
            if (notebook) {
                console.log(`[Analyze] Incident report notebook created: ${notebook.url}`);
            }

            sendJson(res, 200, {
                success: true,
                monitorId,
                monitorName,
                analysis: result,
                similarRCAs: similarRCAs.length,
                templateUsed: template_id || null,
                payloadFields: Object.keys(fullPayload).length,
                notebook: notebook ? { id: notebook.id, url: notebook.url } : null,
                timestamp: new Date().toISOString()
            });

        } catch (err) {
            const errorType = err.errorType || classifyError(err, err.stderr || '');
            const analyzeContext = {
                monitor_id: fullPayload?.monitor_id,
                monitor_name: fullPayload?.monitor_name,
                endpoint: '/analyze'
            };
            structuredLog('error', 'analyze_pipeline_failed', {
                ...analyzeContext,
                error_type: errorType,
                error_message: err.message,
                retries_exhausted: err.retriesExhausted || false
            });

            // Create Datadog failure event (best-effort, don't block response)
            let failureEvent = null;
            let failureNotebook = null;
            try {
                failureEvent = await createFailureEvent(analyzeContext, err);
            } catch (_) { /* best effort */ }

            // Create failure notebook for auth/unknown errors
            if (errorType === 'auth' || errorType === 'unknown') {
                try {
                    failureNotebook = await createFailureNotebook(analyzeContext, err, fullPayload || {});
                } catch (_) { /* best effort */ }
            }

            // Send desktop notification for pipeline failure
            try {
                await sendDesktopNotification({
                    ALERT_STATE: 'RCA_FAILURE',
                    ALERT_TITLE: `RCA Pipeline Failure: ${analyzeContext.monitor_name || 'Unknown'}`,
                    DETAILED_DESCRIPTION: `Error type: ${errorType} - ${err.message}`,
                    URGENCY: 'high'
                });
            } catch (_) { /* best effort */ }

            sendJson(res, 500, {
                error: err.message,
                error_type: errorType,
                retries_exhausted: err.retriesExhausted || false,
                failure_event: failureEvent ? { id: failureEvent.event?.id } : null,
                failure_notebook: failureNotebook ? { id: failureNotebook.id, url: failureNotebook.url } : null,
                timestamp: new Date().toISOString()
            });
        }
        return;
    }

    // Watchdog Analysis endpoint - handles Datadog Watchdog anomaly detection monitors
    // Similar to /analyze but with watchdog-specific prompt and notebook formatting
    if (url.pathname === '/watchdog' && req.method === 'POST') {
        let fullPayload = {};
        try {
            const body = await parseBody(req);
            const { payload } = body;

            fullPayload = payload || body;
            let monitorId = fullPayload.monitor_id || fullPayload.monitorId;
            let monitorName = fullPayload.monitor_name || fullPayload.monitorName;
            const alertStatus = fullPayload.alert_status || fullPayload.alertStatus;
            const hostname = fullPayload.hostname;
            const rawService = fullPayload.service;
            const service = resolveServiceName(fullPayload);
            const scope = fullPayload.scope;
            const tags = fullPayload.tags;
            const applicationTeam = fullPayload.APPLICATION_TEAM || fullPayload.application_team;
            const triggerTime = new Date(fullPayload.timestamp ? fullPayload.timestamp * 1000 : Date.now()).toISOString();

            // Write resolved service back to payload for downstream functions
            fullPayload.service = service;
            console.log(`[Watchdog] Resolved service name: "${service}" (raw: "${rawService || 'N/A'}", APPLICATION_TEAM: "${applicationTeam || 'N/A'}")`);

            // Resolve monitor ID from description URL if not provided
            if (!monitorId && fullPayload.DETAILED_DESCRIPTION) {
                const match = fullPayload.DETAILED_DESCRIPTION.match(/monitors\/(\d+)/);
                if (match) {
                    monitorId = parseInt(match[1], 10);
                }
            }
            if (!monitorId) {
                monitorId = Date.now();
            }

            if (!monitorName) {
                monitorName = fullPayload.ALERT_TITLE || fullPayload.alert_title || 'Watchdog Monitor';
            }

            fullPayload.monitor_id = monitorId;
            fullPayload.monitor_name = monitorName;

            console.log(`[Watchdog] Processing watchdog alert for monitor ${monitorId}: ${monitorName}`);

            // Send desktop notification
            await sendDesktopNotification(fullPayload);

            // Generate embedding for similarity search
            const alertText = `Watchdog Monitor: ${monitorName} Status: ${alertStatus} Host: ${hostname || 'N/A'} Service: ${service || 'N/A'}`;
            let similarRCAs = [];
            const alertEmbedding = await generateEmbeddings(alertText);

            if (alertEmbedding) {
                similarRCAs = await searchSimilarRCAs(alertEmbedding, 3);
                console.log(`[Watchdog] Found ${similarRCAs.length} similar past incidents`);
            }

            // Build similar RCA context
            let similarRCAContext = '';
            if (similarRCAs.length > 0) {
                similarRCAContext = `\n## Similar Past Watchdog/RCA Incidents\n`;
                similarRCAs.forEach((rca, i) => {
                    similarRCAContext += `\n### Past Incident ${i + 1} (similarity: ${(rca.score * 100).toFixed(1)}%)\n`;
                    similarRCAContext += `- Monitor: ${rca.payload.monitor_name}\n`;
                    similarRCAContext += `- Service: ${rca.payload.service || 'N/A'}\n`;
                    similarRCAContext += `- Analysis: ${rca.payload.analysis?.substring(0, 500) || 'N/A'}...\n`;
                });
            }

            // Pre-fetch Datadog data
            console.log(`[Watchdog] Pre-fetching Datadog data for analysis...`);

            const ddBaseUrl = DD_APP_URL;
            const nowTs = Math.floor(Date.now() / 1000) * 1000;
            const thirtyMinAgo = nowTs - (30 * 60 * 1000);

            const datadogUrls = {
                monitor: monitorId ? `${ddBaseUrl}/monitors/${monitorId}` : null,
                host: hostname ? `${ddBaseUrl}/infrastructure?host=${encodeURIComponent(hostname.split('.')[0])}` : null,
                hostDashboard: hostname ? `${ddBaseUrl}/dash/integration/system_overview?tpl_var_host=${encodeURIComponent(hostname.split('.')[0])}` : null,
                logsHost: hostname ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`host:${hostname.split('.')[0]}*`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
                logsService: service ? `${ddBaseUrl}/logs?query=${encodeURIComponent(`service:${service}`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}` : null,
                logsErrors: `${ddBaseUrl}/logs?query=${encodeURIComponent(`status:error`)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
                apmService: service ? `${ddBaseUrl}/apm/services/${service}/operations` : null,
                events: `${ddBaseUrl}/event/explorer?query=${encodeURIComponent('sources:watchdog')}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}`,
                watchdog: `${ddBaseUrl}/watchdog`,
                metrics: hostname ? `${ddBaseUrl}/metric/explorer?exp_metric=system.cpu.user&exp_scope=${encodeURIComponent(`host:${hostname.split('.')[0]}`)}` : null,
            };

            let logsData = null;
            let hostData = null;
            let eventsData = null;
            let monitorData = null;

            // Fetch logs
            try {
                const logQuery = hostname
                    ? `host:${hostname.split('.')[0]}* status:error OR status:warn`
                    : service ? `service:${service} status:error OR status:warn` : `status:error`;
                logsData = await executeDDLibTool('search_logs', {
                    query: logQuery, from_time: 'now-30m', to_time: 'now', limit: 20
                });
                datadogUrls.logsQuery = `${ddBaseUrl}/logs?query=${encodeURIComponent(logQuery)}&from_ts=${thirtyMinAgo}&to_ts=${nowTs}&live=true`;
            } catch (err) {
                console.log(`[Watchdog] Failed to fetch logs: ${err.message}`);
            }

            // Fetch host info
            if (hostname) {
                try {
                    hostData = await executeDDLibTool('get_host_info', { hostname: hostname.split('.')[0] });
                } catch (err) {
                    console.log(`[Watchdog] Failed to fetch host info: ${err.message}`);
                }
            }

            // Fetch events
            try {
                eventsData = await executeDDLibTool('get_events', {
                    from_time: Math.floor(Date.now() / 1000) - 1800,
                    to_time: Math.floor(Date.now() / 1000)
                });
            } catch (err) {
                console.log(`[Watchdog] Failed to fetch events: ${err.message}`);
            }

            // Fetch monitor details
            if (monitorId) {
                try {
                    monitorData = await executeDDLibTool('get_monitor_details', { monitor_id: monitorId });
                } catch (err) {
                    console.log(`[Watchdog] Failed to fetch monitor details: ${err.message}`);
                }
            }

            // Build data context
            let datadogContext = '\n## Live Datadog Data\n';
            datadogContext += `\n**Quick Links:** `;
            datadogContext += `[Watchdog Dashboard](${datadogUrls.watchdog}) | `;
            datadogContext += datadogUrls.monitor ? `[Monitor](${datadogUrls.monitor}) | ` : '';
            datadogContext += datadogUrls.logsQuery ? `[Logs](${datadogUrls.logsQuery}) | ` : '';
            datadogContext += datadogUrls.host ? `[Host](${datadogUrls.host}) | ` : '';
            datadogContext += datadogUrls.events ? `[Events](${datadogUrls.events})` : '';
            datadogContext += `\n`;

            if (logsData && logsData.logs && logsData.logs.length > 0) {
                datadogContext += `\n### Recent Error/Warning Logs\nFound ${logsData.count} relevant entries:\n\n`;
                logsData.logs.slice(0, 10).forEach((log, i) => {
                    datadogContext += `**${i + 1}. [${log.status || 'INFO'}] ${log.timestamp || 'N/A'}**\n`;
                    datadogContext += `- Service: ${log.service || 'unknown'}\n`;
                    datadogContext += `- Host: ${log.host || 'unknown'}\n`;
                    datadogContext += `- Message: ${(log.message || '').substring(0, 300)}\n\n`;
                });
            }

            if (hostData && !hostData.error) {
                datadogContext += `\n### Host: ${hostData.hostname || hostname}\n`;
                datadogContext += `- Status: ${hostData.up ? 'UP' : 'DOWN'}\n`;
                if (hostData.metrics) {
                    datadogContext += `- CPU: ${hostData.metrics.cpu || 'N/A'}%\n`;
                    datadogContext += `- Memory: ${hostData.metrics.memory || 'N/A'}%\n`;
                    datadogContext += `- Load: ${hostData.metrics.load || 'N/A'}\n`;
                }
            }

            if (monitorData && !monitorData.error) {
                datadogContext += `\n### Monitor Configuration\n`;
                datadogContext += `- Type: ${monitorData.type}\n`;
                datadogContext += `- Query: \`${monitorData.query || 'N/A'}\`\n`;
            }

            // Watchdog-specific Claude prompt
            const prompt = `You are an SRE analyzing a **Datadog Watchdog** anomaly detection alert. Watchdog uses AI/ML to automatically detect anomalies in infrastructure metrics, application performance, and log patterns without requiring manual threshold configuration.

## Full Alert Payload
${JSON.stringify(fullPayload, null, 2)}

## Watchdog Alert Summary
- **Monitor Name:** ${monitorName}
- **Monitor ID:** ${monitorId}
- **Alert Status:** ${alertStatus}
- **Triggered At:** ${triggerTime}
- **Hostname:** ${hostname || 'N/A'}
- **Service:** ${service || 'N/A'}
- **Scope:** ${scope || 'N/A'}
- **Application Team:** ${applicationTeam || 'N/A'}
- **Tags:** ${tags?.join(', ') || 'N/A'}

${datadogContext}

${similarRCAContext}

## Watchdog Analysis Instructions

Watchdog alerts indicate ML-detected anomalies that deviate significantly from historical baselines. Analyze this alert with the following focus:

1) **Anomaly Characterization** - What specific anomaly did Watchdog detect? Describe the metric/behavior that deviated from the expected baseline.
2) **Impact Assessment** - What is the blast radius? Which services, hosts, or users are affected?
3) **Correlation Analysis** - Are there correlated anomalies across other metrics, services, or infrastructure? Check the logs and events for related patterns.
4) **Root Cause Hypothesis** - Based on the anomaly pattern and correlated data, what is the most likely underlying cause?
5) **Recommended Actions** - Two specific, actionable steps to investigate or mitigate the anomaly.
6) **Confidence Level** - low/medium/high based on data availability and correlation strength.

Ground your analysis in the live Datadog data provided. Quote specific log entries, metrics, or events as evidence.`;

            const result = await invokeClaudeCode(prompt, WORK_DIR, {
                monitor_id: monitorId,
                monitor_name: monitorName,
                endpoint: '/watchdog'
            });

            // Store in vector DB
            if (alertEmbedding) {
                await storeRCA(monitorId, monitorName, result, alertEmbedding, fullPayload);
                console.log(`[Watchdog] Analysis stored in vector DB`);
            }

            // Create Watchdog-specific notebook
            const notebook = await createWatchdogNotebook(fullPayload, result, triggerTime, similarRCAs, datadogUrls);
            if (notebook) {
                console.log(`[Watchdog] Notebook created: ${notebook.url}`);
            }

            sendJson(res, 200, {
                success: true,
                monitorId,
                monitorName,
                monitorType: 'watchdog',
                analysis: result,
                triggerTime,
                similarRCAs: similarRCAs.length,
                notebook: notebook ? { id: notebook.id, url: notebook.url } : null,
                timestamp: new Date().toISOString()
            });

        } catch (err) {
            const errorType = err.errorType || classifyError(err, err.stderr || '');
            const watchdogContext = {
                monitor_id: fullPayload?.monitor_id,
                monitor_name: fullPayload?.monitor_name,
                endpoint: '/watchdog'
            };
            structuredLog('error', 'watchdog_pipeline_failed', {
                ...watchdogContext,
                error_type: errorType,
                error_message: err.message,
                retries_exhausted: err.retriesExhausted || false
            });

            let failureEvent = null;
            let failureNotebook = null;
            try {
                failureEvent = await createFailureEvent(watchdogContext, err);
            } catch (_) { /* best effort */ }

            if (errorType === 'auth' || errorType === 'unknown') {
                try {
                    failureNotebook = await createFailureNotebook(watchdogContext, err, fullPayload || {});
                } catch (_) { /* best effort */ }
            }

            try {
                await sendDesktopNotification({
                    ALERT_STATE: 'RCA_FAILURE',
                    ALERT_TITLE: `Watchdog Pipeline Failure: ${watchdogContext.monitor_name || 'Unknown'}`,
                    DETAILED_DESCRIPTION: `Error type: ${errorType} - ${err.message}`,
                    URGENCY: 'high'
                });
            } catch (_) { /* best effort */ }

            sendJson(res, 500, {
                error: err.message,
                error_type: errorType,
                retries_exhausted: err.retriesExhausted || false,
                failure_event: failureEvent ? { id: failureEvent.event?.id } : null,
                failure_notebook: failureNotebook ? { id: failureNotebook.id, url: failureNotebook.url } : null,
                timestamp: new Date().toISOString()
            });
        }
        return;
    }

    // Recovery endpoint - handles recovery webhooks and updates existing notebooks
    // When a monitor transitions to OK/Recovered, this finds and resolves the matching notebook
    if (url.pathname === '/recover' && req.method === 'POST') {
        try {
            const body = await parseBody(req);
            const { payload } = body;

            const fullPayload = payload || body;
            const monitorId = fullPayload.monitor_id || fullPayload.monitorId;
            const monitorName = fullPayload.monitor_name || fullPayload.monitorName ||
                fullPayload.ALERT_TITLE || fullPayload.alert_title || 'Unknown Monitor';
            const alertStatus = fullPayload.alert_status || fullPayload.alertStatus || 'OK';
            const recoveryTimestamp = new Date().toISOString();

            console.log(`[Recovery] Recovery event for monitor ${monitorId}: ${monitorName} (status: ${alertStatus})`);

            if (!monitorId) {
                sendJson(res, 400, { error: 'monitor_id is required for recovery' });
                return;
            }

            // Send desktop notification for recovery
            await sendDesktopNotification({
                ...fullPayload,
                ALERT_STATE: 'Recovered',
                ALERT_TITLE: `RECOVERED: ${monitorName}`
            });

            // Look up the notebook in the registry
            const entry = notebookRegistry.get(String(monitorId));
            let notebookResult = null;

            if (entry && entry.notebookId) {
                console.log(`[Recovery] Found notebook ${entry.notebookId} for monitor ${monitorId} (status: ${entry.status})`);
                notebookResult = await resolveNotebook(entry.notebookId, monitorId, monitorName, recoveryTimestamp);
            } else {
                console.log(`[Recovery] No notebook found in registry for monitor ${monitorId}`);
                // Try to find by explicit notebook_id in the payload (for manual recovery)
                const explicitNotebookId = fullPayload.notebook_id || fullPayload.notebookId;
                if (explicitNotebookId) {
                    console.log(`[Recovery] Using explicit notebook_id from payload: ${explicitNotebookId}`);
                    notebookResult = await resolveNotebook(explicitNotebookId, monitorId, monitorName, recoveryTimestamp);
                }
            }

            sendJson(res, 200, {
                success: true,
                monitorId,
                monitorName,
                alertStatus,
                recoveryTimestamp,
                notebookUpdated: !!notebookResult,
                notebookId: entry?.notebookId || null,
                notebookUrl: entry?.notebookId ? `${DD_APP_URL}/notebook/${entry.notebookId}` : null,
                registryStatus: entry?.status || 'not_found',
                timestamp: new Date().toISOString()
            });

        } catch (err) {
            console.error(`[Recovery] Error: ${err.message}`);
            sendJson(res, 500, {
                error: err.message,
                timestamp: new Date().toISOString()
            });
        }
        return;
    }

    // Notebook registry status endpoint - lists all tracked notebooks and their lifecycle status
    if (url.pathname === '/notebooks/registry' && req.method === 'GET') {
        const entries = [];
        for (const [monitorId, entry] of notebookRegistry) {
            entries.push({
                monitorId,
                ...entry,
                notebookUrl: `${DD_APP_URL}/notebook/${entry.notebookId}`
            });
        }
        sendJson(res, 200, {
            count: entries.length,
            notebooks: entries,
            timestamp: new Date().toISOString()
        });
        return;
    }

    // Generate notebook endpoint
    if (url.pathname === '/generate-notebook' && req.method === 'POST') {
        try {
            const body = await parseBody(req);
            const { title, analysis, monitorId } = body;

            if (!title || !analysis) {
                sendJson(res, 400, { error: 'title and analysis are required' });
                return;
            }

            const template = loadTemplate('incident-report-cloned.md');

            const prompt = `Generate a Datadog Notebook based on the following RCA analysis.

## Analysis
${JSON.stringify(analysis, null, 2)}

## Template
${template || 'Use standard incident report format'}

## Requirements
1. Create a notebook JSON structure compatible with Datadog Notebooks API
2. Include relevant queries for logs, metrics, and events
3. Add visualization widgets for key metrics
4. Include the RCA summary and recommendations

Return the notebook JSON that can be POSTed to the Datadog Notebooks API.`;

            const result = await invokeClaudeCode(prompt, WORK_DIR, {
                endpoint: '/generate-notebook'
            });

            sendJson(res, 200, {
                success: true,
                notebook: result,
                timestamp: new Date().toISOString()
            });

        } catch (err) {
            console.error(`[Notebook] Error: ${err.message}`);
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    // List available templates
    if (url.pathname === '/templates' && req.method === 'GET') {
        try {
            const templates = fs.existsSync(ASSETS_DIR)
                ? fs.readdirSync(ASSETS_DIR).filter(f => f.endsWith('.md'))
                : [];
            sendJson(res, 200, { templates });
        } catch (err) {
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    // Execute dd_lib tool endpoint
    if (url.pathname === '/tools/execute' && req.method === 'POST') {
        try {
            const body = await parseBody(req);
            const { tool, params } = body;

            if (!tool) {
                sendJson(res, 400, { error: 'tool name is required' });
                return;
            }

            const result = await executeDDLibTool(tool, params || {});
            sendJson(res, 200, {
                success: true,
                tool,
                result,
                timestamp: new Date().toISOString()
            });
        } catch (err) {
            console.error(`[Tools] Error executing ${req.body?.tool}: ${err.message}`);
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    // Create dd_lib function endpoint (auto-write mode)
    if (url.pathname === '/tools/create-function' && req.method === 'POST') {
        try {
            const body = await parseBody(req);
            const { module_name, function_code } = body;

            if (!module_name || !function_code) {
                sendJson(res, 400, { error: 'module_name and function_code are required' });
                return;
            }

            const result = await createDDLibFunction(module_name, function_code);
            sendJson(res, 200, {
                success: true,
                module: module_name,
                path: result.path,
                timestamp: new Date().toISOString()
            });
        } catch (err) {
            console.error(`[Tools] Error creating function: ${err.message}`);
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    // List available dd_lib tools
    if (url.pathname === '/tools' && req.method === 'GET') {
        sendJson(res, 200, {
            tools: [
                { name: 'get_monitors', description: 'List all monitors', params: [] },
                { name: 'get_triggered_monitors', description: 'Get triggered monitors', params: ['limit'] },
                { name: 'get_host_info', description: 'Get host details', params: ['hostname'] },
                { name: 'search_logs', description: 'Search logs', params: ['query', 'from_time', 'to_time'] },
                { name: 'get_events', description: 'Get events', params: ['from_time', 'to_time'] },
                { name: 'create_function', description: 'Create new dd_lib function', params: ['module_name', 'function_code'] }
            ],
            ddLibPath: DD_LIB_DIR,
            writable: true
        });
        return;
    }

    // GoNotebook re-ingestion endpoint
    if (url.pathname === '/go-principles/reingest' && req.method === 'POST') {
        try {
            console.log('[GoNotebook] Re-ingestion requested');
            // Delete existing collection
            await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}`, 'DELETE');
            // Re-create and ingest
            await initGoPrinciplesCollection();
            const result = await ingestGoNotebook();
            sendJson(res, 200, { status: 'reingested', ...result });
        } catch (err) {
            console.error(`[GoNotebook] Re-ingestion error: ${err.message}`);
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    // GoNotebook stats endpoint
    if (url.pathname === '/go-principles/stats' && req.method === 'GET') {
        try {
            const collectionResp = await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}`, 'GET');
            if (collectionResp.status !== 200) {
                sendJson(res, 200, { exists: false, pointCount: 0, status: 'not_initialized' });
                return;
            }
            const countResp = await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}/points/count`, 'POST', {});
            const pointCount = countResp.data?.result?.count || 0;

            // Get a sample of points for inspection
            let samplePoints = [];
            try {
                const scrollResp = await httpRequest(`${QDRANT_URL}/collections/${GO_PRINCIPLES_COLLECTION}/points/scroll`, 'POST', {
                    limit: 5,
                    with_payload: true,
                    with_vector: false
                });
                if (scrollResp.data?.result?.points) {
                    samplePoints = scrollResp.data.result.points.map(p => ({
                        id: p.id,
                        category: p.payload.category,
                        topic: p.payload.topic,
                        file_path: p.payload.file_path,
                        section_heading: p.payload.section_heading,
                        text_preview: (p.payload.text || '').substring(0, 200)
                    }));
                }
            } catch (scrollErr) {
                // Scroll is optional, don't fail on it
            }

            sendJson(res, 200, {
                exists: true,
                pointCount,
                gonotebookPath: GONOTEBOOK_PATH,
                pathExists: fs.existsSync(GONOTEBOOK_PATH),
                samplePoints
            });
        } catch (err) {
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    // GitHub Issue Processing endpoint
    if (url.pathname === '/github/process-issue' && req.method === 'POST') {
        try {
            // Verify internal shared secret
            const expectedSecret = process.env.AGENT_INTERNAL_SECRET;
            if (expectedSecret) {
                const provided = req.headers['x-agent-secret'];
                if (provided !== expectedSecret) {
                    sendJson(res, 401, { error: 'unauthorized' });
                    return;
                }
            }

            const body = await parseBody(req);
            const { issue_number, issue_title, issue_body, repo_name, sender_login, event_id } = body;

            if (!issue_number || !issue_title || !repo_name) {
                sendJson(res, 400, { error: 'issue_number, issue_title, and repo_name are required' });
                return;
            }

            console.log(`[GitHub] Processing issue #${issue_number}: ${issue_title}`);

            // Invoke Claude Code to implement the feature
            const summary = await processGitHubIssue(body);

            // Check if this was detected as a duplicate
            if (summary.duplicate) {
                console.log(`[GitHub] Issue #${issue_number} detected as duplicate of #${summary.duplicate_of}`);

                // Build duplicate comment
                const dupComment = `## Duplicate Issue Detected\n\n` +
                    `This feature request appears to have already been implemented in **issue #${summary.duplicate_of}**.\n\n` +
                    `${summary.summary || ''}\n\n` +
                    `Closing as duplicate. If this is incorrect, please reopen with additional context explaining what differs.\n\n` +
                    `---\n*Automated by Rayne Claude Agent*`;

                try {
                    await closeGitHubIssue(repo_name, issue_number, dupComment);
                    summary.comment_url = 'closed-as-duplicate';
                } catch (ghErr) {
                    console.error(`[GitHub] Failed to close duplicate issue: ${ghErr.message}`);
                    summary.comment_error = ghErr.message;
                }

                sendJson(res, 200, {
                    success: true,
                    duplicate: true,
                    duplicate_of: summary.duplicate_of,
                    summary: summary.summary || '',
                    comment_url: summary.comment_url || '',
                    timestamp: new Date().toISOString()
                });
                return;
            }

            // Not a duplicate - comment on the GitHub issue with results
            try {
                const commentBody = buildIssueComment(summary);
                const commentUrl = await commentOnGitHubIssue(repo_name, issue_number, commentBody);
                summary.comment_url = commentUrl;
            } catch (ghErr) {
                console.error(`[GitHub] Failed to comment on issue: ${ghErr.message}`);
                summary.comment_error = ghErr.message;
            }

            sendJson(res, 200, {
                success: true,
                branch_name: summary.branch_name || '',
                modified_files: summary.modified_files || [],
                summary: summary.summary || '',
                comment_url: summary.comment_url || '',
                timestamp: new Date().toISOString()
            });
        } catch (err) {
            console.error(`[GitHub] Processing error: ${err.message}`);
            sendJson(res, 500, { success: false, error: 'internal processing error' });
        }
        return;
    }

    // 404 for unknown routes
    sendJson(res, 404, { error: 'Not found' });
});

server.listen(PORT, async () => {
    console.log(`[Claude Agent] Server listening on port ${PORT}`);
    console.log(`[Claude Agent] dd_lib path: ${DD_LIB_DIR}`);
    console.log(`[Claude Agent] assets path: ${ASSETS_DIR}`);
    console.log(`[Claude Agent] work dir: ${WORK_DIR}`);
    console.log(`[Claude Agent] Qdrant URL: ${QDRANT_URL}`);
    console.log(`[Claude Agent] Ollama URL: ${OLLAMA_URL}`);

    // Initialize Qdrant collections on startup
    setTimeout(async () => {
        // Existing RCA collection init
        const initialized = await initQdrantCollection();
        console.log(`[Claude Agent] Qdrant RCA collection initialized: ${initialized}`);

        // GoNotebook collection init + conditional ingestion
        try {
            const goCollection = await initGoPrinciplesCollection();
            console.log(`[Claude Agent] Go principles collection: ${JSON.stringify(goCollection)}`);

            if (goCollection.exists && goCollection.pointCount > 0) {
                console.log(`[Claude Agent] Go principles already ingested (${goCollection.pointCount} points), skipping`);
            } else {
                console.log(`[Claude Agent] Starting goNotebook ingestion...`);
                const result = await ingestGoNotebook();
                console.log(`[Claude Agent] GoNotebook ingestion complete: ${JSON.stringify(result)}`);
            }
        } catch (err) {
            console.error(`[Claude Agent] GoNotebook init error: ${err.message}`);
        }
    }, 5000); // Wait 5 seconds for Qdrant to be ready
});
