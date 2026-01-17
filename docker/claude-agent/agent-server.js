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

// Store RCA in Qdrant vector DB
async function storeRCA(monitorId, monitorName, analysis, embedding) {
    try {
        const point = {
            id: Date.now(),
            vector: embedding,
            payload: {
                monitor_id: monitorId,
                monitor_name: monitorName,
                analysis: analysis,
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

    // RCA Analysis endpoint
    if (url.pathname === '/analyze' && req.method === 'POST') {
        try {
            const body = await parseBody(req);
            const { monitorId, alertStatus, monitorName, scope, tags } = body;

            if (!monitorId || !monitorName) {
                sendJson(res, 400, { error: 'monitorId and monitorName are required' });
                return;
            }

            // Load incident report template
            const template = loadTemplate('incident-report-cloned.md');
            const logsTemplate = loadTemplate('logs-analysis.md');

            // Generate embedding for the alert to find similar past RCAs
            const alertText = `Monitor: ${monitorName} Status: ${alertStatus} Scope: ${scope || 'N/A'}`;
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
                    similarRCAContext += `- Analysis: ${rca.payload.analysis?.substring(0, 500) || 'N/A'}...\n`;
                });
            }

            const prompt = `You are an SRE. Analyze this Datadog alert briefly: Alert: ${monitorName}, Status: ${alertStatus}, Scope: ${scope || 'N/A'}. Provide: 1) Likely root cause 2) Confidence (low/medium/high) 3) Two recommendations. Keep response under 100 words.`;

            console.log(`[Analyze] Processing alert for monitor ${monitorId}: ${monitorName}`);

            const result = await invokeClaudeCode(prompt);

            // Store this RCA in vector DB for future reference
            if (alertEmbedding) {
                const stored = await storeRCA(monitorId, monitorName, result, alertEmbedding);
                console.log(`[Analyze] RCA stored in vector DB: ${stored}`);
            }

            sendJson(res, 200, {
                success: true,
                monitorId,
                monitorName,
                analysis: result,
                similarRCAs: similarRCAs.length,
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
