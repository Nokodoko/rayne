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

// Datadog API configuration
const DD_API_KEY = process.env.DD_API_KEY;
const DD_APP_KEY = process.env.DD_APP_KEY;
const DD_API_URL = process.env.DD_API_URL || 'https://api.ddog-gov.com';

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

// Make HTTP request helper
function httpRequest(url, method, data = null) {
    return new Promise((resolve, reject) => {
        const urlObj = new URL(url);
        const options = {
            hostname: urlObj.hostname,
            port: urlObj.port,
            path: urlObj.pathname + urlObj.search,
            method: method,
            headers: { 'Content-Type': 'application/json' }
        };

        const req = http.request(options, (res) => {
            let body = '';
            res.on('data', chunk => body += chunk);
            res.on('end', () => {
                try {
                    resolve({ status: res.statusCode, data: body ? JSON.parse(body) : null });
                } catch (e) {
                    resolve({ status: res.statusCode, data: body });
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
    const service = payload.service || 'N/A';
    const scope = payload.scope || 'N/A';
    const tags = payload.tags || [];
    const applicationTeam = payload.APPLICATION_TEAM || payload.application_team || 'N/A';
    const timestamp = new Date().toISOString();

    // Build default URLs if not provided
    const ddBaseUrl = 'https://app.datadoghq.com';
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
        similarRCAsMarkdown = `\n\n## Similar Past Incidents\n\n`;
        similarRCAs.forEach((rca, i) => {
            const rcaMonitorId = rca.payload?.monitor_id;
            const rcaMonitorName = rca.payload?.monitor_name || 'Unknown';
            const rcaMonitorUrl = rcaMonitorId ? `${ddBaseUrl}/monitors/${rcaMonitorId}` : null;
            similarRCAsMarkdown += `### ${i + 1}. ${rcaMonitorUrl ? `[${rcaMonitorName}](${rcaMonitorUrl})` : rcaMonitorName} (${(rca.score * 100).toFixed(0)}% similar)\n`;
            similarRCAsMarkdown += `${rca.payload?.analysis?.substring(0, 300) || 'No analysis available'}...\n\n`;
        });
    }

    // Build quick links section
    let quickLinksMarkdown = '### 🔗 Quick Links\n\n';
    if (urls.monitor) quickLinksMarkdown += `- [📊 View Monitor](${urls.monitor})\n`;
    if (urls.logsHost || urls.logsService) quickLinksMarkdown += `- [📋 View Logs](${urls.logsHost || urls.logsService || urls.logsErrors})\n`;
    if (urls.apmService) quickLinksMarkdown += `- [🔍 APM Service](${urls.apmService})\n`;
    if (urls.apmTraces) quickLinksMarkdown += `- [🔗 APM Traces](${urls.apmTraces})\n`;
    if (urls.apmErrors) quickLinksMarkdown += `- [⚠️ Error Traces](${urls.apmErrors})\n`;
    if (urls.host) quickLinksMarkdown += `- [🖥️ Host Infrastructure](${urls.host})\n`;
    if (urls.hostDashboard) quickLinksMarkdown += `- [📈 Host Dashboard](${urls.hostDashboard})\n`;
    if (urls.metrics) quickLinksMarkdown += `- [📉 Metrics Explorer](${urls.metrics})\n`;
    if (urls.events) quickLinksMarkdown += `- [📅 Events](${urls.events})\n`;
    if (urls.dbm) quickLinksMarkdown += `- [🗄️ Database Monitoring](${urls.dbm})\n`;
    if (urls.dbmQueries) quickLinksMarkdown += `- [💾 DB Queries](${urls.dbmQueries})\n`;

    // Build header with hyperlinks in table
    const headerMarkdown = `# Incident Report: ${monitorName}\n\n` +
        `**Generated:** ${timestamp}\n\n` +
        `---\n\n` +
        `| Field | Value |\n` +
        `|-------|-------|\n` +
        `| Monitor ID | ${urls.monitor ? `[${monitorId}](${urls.monitor})` : monitorId} |\n` +
        `| Alert Status | **${alertStatus}** |\n` +
        `| Hostname | ${urls.host ? `[${hostname}](${urls.host})` : hostname} |\n` +
        `| Service | ${urls.apmService ? `[${service}](${urls.apmService})` : service} |\n` +
        `| Scope | ${scope} |\n` +
        `| Application Team | ${applicationTeam} |\n` +
        `| Tags | ${tags.join(', ') || 'N/A'} |\n\n` +
        quickLinksMarkdown;

    const notebookData = {
        data: {
            type: "notebooks",
            attributes: {
                name: `[Incident Report] ${monitorName} - ${timestamp.split('T')[0]}`,
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
                    // Analysis cell
                    {
                        type: "notebook_cells",
                        attributes: {
                            definition: {
                                type: "markdown",
                                text: `## Root Cause Analysis\n\n${analysis}`
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
                                    `*This incident report was automatically generated by Rayne Claude Agent.*\n\n` +
                                    `**Actions:**\n` +
                                    `${urls.monitor ? `- [View Monitor](${urls.monitor})\n` : ''}` +
                                    `${urls.monitor ? `- [Edit Monitor](${urls.monitor}/edit)\n` : ''}` +
                                    `${urls.events ? `- [View Related Events](${urls.events})\n` : ''}`
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
            const notebookUrl = `${DD_API_URL.replace('api.', 'app.')}/notebook/${notebookId}`;
            console.log(`[Notebook] Created successfully: ${notebookUrl}`);
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

// Invoke Claude Code CLI with a prompt
function invokeClaudeCode(prompt, workDir = WORK_DIR) {
    return new Promise((resolve, reject) => {
        const args = ['--print', prompt];

        console.log(`[Claude] Invoking with prompt: ${prompt.substring(0, 100)}...`);

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
            console.error(`[Claude stderr] ${data.toString()}`);
        });

        claude.on('close', code => {
            console.log(`[Claude] Exited with code ${code}`);
            if (code === 0) {
                resolve(stdout);
            } else {
                reject(new Error(`Claude exited with code ${code}: ${stderr}`));
            }
        });

        claude.on('error', err => {
            reject(new Error(`Failed to spawn Claude: ${err.message}`));
        });
    });
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
        try {
            const body = await parseBody(req);
            const { payload, template_id, instructions } = body;

            // Support both new format (payload object) and legacy format
            const fullPayload = payload || body;
            const monitorId = fullPayload.monitor_id || fullPayload.monitorId;
            const monitorName = fullPayload.monitor_name || fullPayload.monitorName;
            const alertStatus = fullPayload.alert_status || fullPayload.alertStatus;
            const scope = fullPayload.scope;
            const tags = fullPayload.tags;
            const hostname = fullPayload.hostname;
            const service = fullPayload.service;
            const applicationTeam = fullPayload.APPLICATION_TEAM || fullPayload.application_team;

            if (!monitorId || !monitorName) {
                sendJson(res, 400, { error: 'monitorId and monitorName are required in payload' });
                return;
            }

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
            const ddBaseUrl = 'https://app.datadoghq.com';
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

## Analysis Instructions
Based on the LIVE Datadog data above (logs, host info, events, monitor config), provide:

1) **Root Cause Analysis** - What is the most likely cause? Cite specific evidence from the logs or events.
2) **Confidence Level** - low/medium/high with reasoning based on available data
3) **Immediate Actions** - Two specific recommendations to mitigate the issue
4) **Related Impact** - Other services/hosts that may be affected based on the data

If logs show specific errors, quote them. If host metrics are abnormal, mention them. Ground your analysis in the actual data provided.`;

            console.log(`[Analyze] Processing alert for monitor ${monitorId}: ${monitorName}`);
            console.log(`[Analyze] Full payload received with ${Object.keys(fullPayload).length} fields`);

            const result = await invokeClaudeCode(prompt);

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
            console.error(`[Analyze] Error: ${err.message}`);
            sendJson(res, 500, {
                error: err.message,
                timestamp: new Date().toISOString()
            });
        }
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

            const result = await invokeClaudeCode(prompt);

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

    // Initialize Qdrant collection on startup
    setTimeout(async () => {
        const initialized = await initQdrantCollection();
        console.log(`[Claude Agent] Qdrant collection initialized: ${initialized}`);
    }, 5000); // Wait 5 seconds for Qdrant to be ready
});
