#!/usr/bin/env node

/**
 * Headless Browser Traffic Generator for Rayne RUM
 *
 * Uses Puppeteer to generate real browser traffic with the actual Datadog RUM SDK.
 * Simulates both new and returning visitors with realistic browsing patterns.
 *
 * This ensures traffic looks exactly like real browser traffic since it comes from
 * an actual browser instance with the SDK properly initialized.
 */

const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');
puppeteer.use(StealthPlugin());
const fs = require('fs');
const path = require('path');
const http = require('http');
const https = require('https');

// Configuration with defaults
const config = {
  frontendUrl: process.env.FRONTEND_URL || 'http://localhost:3000',
  backendUrl: process.env.BACKEND_URL || 'http://localhost:8080',
  sessions: parseInt(process.env.SESSIONS || '10'),
  concurrent: parseInt(process.env.CONCURRENT || '2'),
  newUserRate: parseInt(process.env.NEW_USER_RATE || '25'),
  maxPoolSize: parseInt(process.env.MAX_POOL_SIZE || '100'),
  poolFile: process.env.POOL_FILE || '/tmp/rayne-visitor-pool-headless.json',
  headless: process.env.HEADLESS !== 'false',
  verbose: process.env.VERBOSE === 'true',
  continuous: process.env.CONTINUOUS === 'true',
  minDelay: parseInt(process.env.MIN_DELAY || '1000'),
  maxDelay: parseInt(process.env.MAX_DELAY || '5000'),
};

const HELP_TEXT = `
Headless Browser Traffic Generator for Rayne RUM

USAGE:
  node headless-traffic-generator.js [OPTIONS]

OPTIONS:
  --frontend-url <url>   Frontend URL (default: http://localhost:3000)
  --backend-url <url>    Backend URL for RUM init (default: http://localhost:8080)
  --sessions <n>         Number of sessions to generate (default: 10)
  --concurrent <n>       Concurrent browser instances (default: 2)
  --new-user-rate <n>    Percentage of new vs returning visitors (default: 25)
  --max-pool-size <n>    Maximum visitor pool size (default: 100)
  --pool-file <path>     Path to visitor pool file (default: /tmp/rayne-visitor-pool-headless.json)
  --min-delay <ms>       Minimum delay between sessions (default: 2000)
  --max-delay <ms>       Maximum delay between sessions (default: 10000)
  --headless             Run in headless mode (default)
  --no-headless          Show browser window
  --verbose              Show detailed logs
  --continuous           Run continuously until stopped
  --help, -h             Show this help message

ENVIRONMENT VARIABLES:
  FRONTEND_URL           Same as --frontend-url
  BACKEND_URL            Same as --backend-url
  SESSIONS               Same as --sessions
  CONCURRENT             Same as --concurrent
  NEW_USER_RATE          Same as --new-user-rate
  MAX_POOL_SIZE          Same as --max-pool-size
  POOL_FILE              Same as --pool-file
  MIN_DELAY              Same as --min-delay
  MAX_DELAY              Same as --max-delay
  HEADLESS               Set to 'false' to show browser
  VERBOSE                Set to 'true' for detailed logs
  CONTINUOUS             Set to 'true' for continuous mode

EXAMPLES:
  # Generate 20 sessions with 50% new users
  node headless-traffic-generator.js --sessions 20 --new-user-rate 50

  # Run continuously with visible browser
  node headless-traffic-generator.js --continuous --no-headless --verbose

  # Use environment variables
  SESSIONS=50 NEW_USER_RATE=30 node headless-traffic-generator.js

  # Quick test with 3 visible sessions
  npm run traffic:visible
`;

// Parse CLI arguments
for (let i = 2; i < process.argv.length; i++) {
  const arg = process.argv[i];
  if (arg === '--help' || arg === '-h') {
    console.log(HELP_TEXT);
    process.exit(0);
  } else if (arg === '--frontend-url' && process.argv[i + 1]) {
    config.frontendUrl = process.argv[++i];
  } else if (arg === '--backend-url' && process.argv[i + 1]) {
    config.backendUrl = process.argv[++i];
  } else if (arg === '--sessions' && process.argv[i + 1]) {
    config.sessions = parseInt(process.argv[++i]);
  } else if (arg === '--concurrent' && process.argv[i + 1]) {
    config.concurrent = parseInt(process.argv[++i]);
  } else if (arg === '--new-user-rate' && process.argv[i + 1]) {
    config.newUserRate = parseInt(process.argv[++i]);
  } else if (arg === '--max-pool-size' && process.argv[i + 1]) {
    config.maxPoolSize = parseInt(process.argv[++i]);
  } else if (arg === '--pool-file' && process.argv[i + 1]) {
    config.poolFile = process.argv[++i];
  } else if (arg === '--min-delay' && process.argv[i + 1]) {
    config.minDelay = parseInt(process.argv[++i]);
  } else if (arg === '--max-delay' && process.argv[i + 1]) {
    config.maxDelay = parseInt(process.argv[++i]);
  } else if (arg === '--headless') {
    config.headless = true;
  } else if (arg === '--no-headless') {
    config.headless = false;
  } else if (arg === '--verbose') {
    config.verbose = true;
  } else if (arg === '--continuous') {
    config.continuous = true;
  }
}

// Page sections to visit
const pageSections = ['/', '/#about', '/#projects', '/#contact'];

// User agents for variety (Chrome/122+ for current versions)
const userAgents = [
  { ua: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36', os: 'Windows', browser: 'Chrome' },
  { ua: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15', os: 'macOS', browser: 'Safari' },
  { ua: 'Mozilla/5.0 (X11; Linux x86_64; rv:123.0) Gecko/20100101 Firefox/123.0', os: 'Linux', browser: 'Firefox' },
  { ua: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0', os: 'Windows', browser: 'Edge' },
  { ua: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/604.1', os: 'iOS', browser: 'Safari Mobile' },
  { ua: 'Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36', os: 'Android', browser: 'Chrome Mobile' },
];

// Viewport sizes for variety
const viewports = [
  { width: 1920, height: 1080, isMobile: false, name: 'Desktop HD' },
  { width: 1366, height: 768, isMobile: false, name: 'Desktop' },
  { width: 1536, height: 864, isMobile: false, name: 'Laptop' },
  { width: 1440, height: 900, isMobile: false, name: 'MacBook' },
  { width: 390, height: 844, isMobile: true, name: 'iPhone 14' },
  { width: 412, height: 915, isMobile: true, name: 'Pixel 7' },
  { width: 768, height: 1024, isMobile: true, name: 'iPad' },
];

// Referrers for realistic traffic sources
const referrers = [
  'https://www.google.com/search?q=portfolio',
  'https://github.com/Nokodoko',
  'https://www.linkedin.com/',
  'https://twitter.com/',
  '', // Direct traffic
  '', // Direct traffic
];

// Action types to simulate
const actionTypes = ['button_click', 'link_hover', 'scroll', 'form_focus', 'navigation'];

// Visitor pool management with cookie persistence
let visitorPool = {};  // { uuid: { cookies: [], userAgent: '', viewport: {} } }

const COOKIE_FILE = config.poolFile.replace('.json', '-cookies.json');

function loadPool() {
  try {
    if (fs.existsSync(config.poolFile)) {
      const data = fs.readFileSync(config.poolFile, 'utf8');
      const parsed = JSON.parse(data);
      // Handle both old format (array) and new format (object)
      if (Array.isArray(parsed)) {
        // Convert old format to new
        parsed.forEach(uuid => {
          visitorPool[uuid] = { cookies: [], userAgent: null, viewport: null };
        });
      } else {
        visitorPool = parsed;
      }
      log(`Loaded ${Object.keys(visitorPool).length} visitors from pool`);
    }
  } catch (e) {
    visitorPool = {};
  }
}

function savePool() {
  try {
    fs.writeFileSync(config.poolFile, JSON.stringify(visitorPool, null, 2));
  } catch (e) {
    // Ignore save errors
  }
}

function addToPool(visitorUuid, cookies = [], userAgent = null, viewport = null) {
  if (!visitorUuid) return;

  visitorPool[visitorUuid] = { cookies, userAgent, viewport };

  // Trim pool if exceeds max size
  const uuids = Object.keys(visitorPool);
  if (uuids.length > config.maxPoolSize) {
    const toRemove = uuids.slice(0, uuids.length - config.maxPoolSize);
    toRemove.forEach(uuid => delete visitorPool[uuid]);
  }

  savePool();
}

function updateVisitorCookies(visitorUuid, cookies) {
  if (visitorPool[visitorUuid]) {
    visitorPool[visitorUuid].cookies = cookies;
    savePool();
  }
}

function getPoolVisitor() {
  const uuids = Object.keys(visitorPool);
  if (uuids.length === 0) return null;
  const uuid = uuids[Math.floor(Math.random() * uuids.length)];
  return { uuid, ...visitorPool[uuid] };
}

// Utility functions
function log(message, ...args) {
  const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
  console.log(`[${timestamp}] ${message}`, ...args);
}

function verboseLog(message, ...args) {
  if (config.verbose) {
    log(`  ${message}`, ...args);
  }
}

function randomElement(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

// Wrap any promise with a timeout
function withTimeout(promise, ms, fallback = null) {
  return Promise.race([
    promise,
    new Promise(resolve => setTimeout(() => resolve(fallback), ms))
  ]);
}

/**
 * Apply stealth evasions to a page to avoid bot detection
 */
async function applyStealthEvasions(page) {
  await page.evaluateOnNewDocument(() => {
    Object.defineProperty(navigator, 'webdriver', { get: () => false, configurable: true });
    Object.defineProperty(navigator, 'plugins', {
      get: () => {
        const plugins = [
          { name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
          { name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '' },
          { name: 'Native Client', filename: 'internal-nacl-plugin', description: '' },
        ];
        plugins.length = 3;
        return plugins;
      },
    });
    Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
    window.chrome = { runtime: { connect: () => {}, sendMessage: () => {}, onMessage: { addListener: () => {} } } };
  });
}

/**
 * Initialize RUM session with backend
 */
async function initRumSession(visitorUuid, userAgent, referrer) {
  return new Promise((resolve, reject) => {
    const url = new URL(`${config.backendUrl}/v1/rum/init`);
    const isHttps = url.protocol === 'https:';

    const payload = JSON.stringify({
      existing_uuid: visitorUuid || undefined,
      user_agent: userAgent,
      referrer: referrer,
      page_url: config.frontendUrl,
    });

    const options = {
      hostname: url.hostname,
      port: url.port || (isHttps ? 443 : 80),
      path: url.pathname,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(payload),
        'User-Agent': userAgent,
      },
    };

    const req = (isHttps ? https : http).request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          resolve(JSON.parse(data));
        } catch (e) {
          reject(new Error(`Invalid JSON response: ${data}`));
        }
      });
    });

    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

/**
 * Track RUM event with backend
 */
async function trackRumEvent(sessionId, eventType, eventData, userAgent) {
  return new Promise((resolve, reject) => {
    const url = new URL(`${config.backendUrl}/v1/rum/track`);
    const isHttps = url.protocol === 'https:';

    const payload = JSON.stringify({
      session_id: sessionId,
      event_type: eventType,
      event_data: eventData,
    });

    const options = {
      hostname: url.hostname,
      port: url.port || (isHttps ? 443 : 80),
      path: url.pathname,
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(payload),
        'User-Agent': userAgent,
      },
    };

    const req = (isHttps ? https : http).request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => resolve(data));
    });

    req.on('error', reject);
    req.write(payload);
    req.end();
  });
}

/**
 * Simulate a single user session
 */
async function simulateSession(browser, sessionNum) {
  const referrer = randomElement(referrers);

  // Determine new vs returning visitor
  const poolSize = Object.keys(visitorPool).length;
  const isNewUser = Math.random() * 100 < config.newUserRate || poolSize === 0;
  const existingVisitor = isNewUser ? null : getPoolVisitor();

  // For returning visitors, use their saved user agent and viewport if available
  const userAgentInfo = existingVisitor?.userAgent
    ? userAgents.find(ua => ua.ua === existingVisitor.userAgent) || randomElement(userAgents)
    : randomElement(userAgents);
  const viewport = existingVisitor?.viewport || randomElement(viewports);

  const visitorType = isNewUser ? 'NEW' : 'RETURNING';
  log(`Session ${sessionNum}: ${visitorType} visitor (${viewport.name}, ${userAgentInfo.browser})`);

  const context = await browser.createBrowserContext();
  const page = await context.newPage();
  await applyStealthEvasions(page);

  await page.setUserAgent(userAgentInfo.ua);
  await page.setViewport({ width: viewport.width, height: viewport.height, isMobile: viewport.isMobile });

  // For returning visitors, restore their cookies to be recognized by Datadog
  if (existingVisitor?.cookies?.length > 0) {
    try {
      await page.setCookie(...existingVisitor.cookies);
      verboseLog(`Restored ${existingVisitor.cookies.length} cookies for returning visitor`);
    } catch (e) {
      verboseLog(`Failed to restore cookies: ${e.message}`);
    }
  }

  // Listen for Datadog SDK network requests
  if (config.verbose) {
    page.on('request', request => {
      if (request.url().includes('browser-intake')) {
        verboseLog(`[DD Request] ${request.method()} ${request.url().substring(0, 80)}...`);
      }
    });
  }

  let visitorUuid = existingVisitor?.uuid || null;
  let sessionId = null;
  let isNew = isNewUser;

  try {
    // Initialize RUM session with backend BEFORE navigating
    verboseLog(`Initializing RUM session with backend...`);
    try {
      const initResponse = await initRumSession(visitorUuid, userAgentInfo.ua, referrer);
      visitorUuid = initResponse.visitor_uuid;
      sessionId = initResponse.session_id;
      isNew = initResponse.is_new;

      verboseLog(`Got visitor_uuid: ${visitorUuid?.substring(0, 8)}..., session_id: ${sessionId?.substring(0, 8)}..., is_new: ${isNew}`);
    } catch (e) {
      verboseLog(`Backend init failed (continuing anyway): ${e.message}`);
      // Generate a UUID if backend fails
      if (!visitorUuid) {
        visitorUuid = require('crypto').randomUUID();
        isNew = true;
      }
    }

    // CRITICAL: Inject script to set user BEFORE DD_RUM initializes
    // This ensures the user ID is set on the very first event
    await page.evaluateOnNewDocument((uuid) => {
      // Set up a MutationObserver to catch when DD_RUM becomes available
      const setUserWhenReady = () => {
        if (window.DD_RUM && window.DD_RUM.setUser) {
          window.DD_RUM.setUser({ id: uuid, name: `Visitor ${uuid.substring(0, 8)}` });
          return true;
        }
        return false;
      };

      // Try immediately
      if (!setUserWhenReady()) {
        // Poll for DD_RUM to be available
        const interval = setInterval(() => {
          if (setUserWhenReady()) {
            clearInterval(interval);
          }
        }, 50);

        // Stop polling after 10 seconds
        setTimeout(() => clearInterval(interval), 10000);
      }

      // Also store the UUID for any custom tracking
      window.__RAYNE_VISITOR_UUID__ = uuid;
    }, visitorUuid);

    // Navigate to frontend
    verboseLog(`Navigating to ${config.frontendUrl}`);
    await page.goto(config.frontendUrl, {
      waitUntil: 'domcontentloaded',
      timeout: 15000
    });

    // Wait for Datadog RUM SDK to initialize and user to be set
    await sleep(2000);

    // Verify DD_RUM is loaded and user is set
    const ddStatus = await withTimeout(
      page.evaluate(() => {
        const hasRum = typeof window.DD_RUM !== 'undefined';
        let userId = null;
        if (hasRum && window.DD_RUM.getUser) {
          const user = window.DD_RUM.getUser();
          userId = user?.id;
        }
        return { hasRum, userId };
      }),
      5000,
      { hasRum: false, userId: null }
    );

    if (!ddStatus.hasRum) {
      log(`  Session ${sessionNum}: Warning - DD_RUM not detected on page`);
    } else {
      verboseLog(`DD_RUM user set to: ${ddStatus.userId?.substring(0, 8) || 'none'}...`);
    }

    // Simulate browsing - visit 2-4 sections (simplified for reliability)
    const numPages = randomInt(2, 4);
    const sectionsToVisit = ['/'];

    while (sectionsToVisit.length < numPages) {
      const section = randomElement(pageSections);
      if (!sectionsToVisit.includes(section)) {
        sectionsToVisit.push(section);
      }
    }

    for (let i = 0; i < sectionsToVisit.length; i++) {
      const section = sectionsToVisit[i];
      const url = section === '/' ? config.frontendUrl : `${config.frontendUrl}${section}`;
      verboseLog(`Visiting ${section}`);

      if (i > 0) {
        try {
          await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 10000 });
        } catch (e) {
          verboseLog(`Navigation failed: ${e.message}`);
          break; // Skip remaining pages if navigation fails
        }
      }

      // Track page view with backend (fire and forget)
      if (sessionId) {
        trackRumEvent(sessionId, 'page_view', {
          page_url: url,
          page_title: section === '/' ? 'Home' : section.replace('/#', '').charAt(0).toUpperCase() + section.replace('/#', '').slice(1),
        }, userAgentInfo.ua).catch(() => {});
      }

      // Simulate reading time (1-3 seconds)
      await sleep(randomInt(1000, 3000));

      // Simple scroll (fire and forget with timeout)
      page.evaluate(() => window.scrollBy(0, Math.random() * 500)).catch(() => {});
      await sleep(500);
    }

    // Wait for events to be flushed to Datadog
    await sleep(2000);

    // Force Datadog SDK to flush before closing
    await page.evaluate(() => {
      if (window.DD_RUM && window.DD_RUM.stopSession) {
        window.DD_RUM.stopSession();
      }
    }).catch(() => {});
    await sleep(1000); // Give time for flush

    // Save cookies for this visitor (for future returning visits)
    try {
      const cookies = await withTimeout(page.cookies(), 3000, []);
      if (visitorUuid && cookies.length > 0) {
        if (isNew) {
          addToPool(visitorUuid, cookies, userAgentInfo.ua, viewport);
          verboseLog(`Saved new visitor with ${cookies.length} cookies`);
        } else {
          updateVisitorCookies(visitorUuid, cookies);
          verboseLog(`Updated cookies for returning visitor`);
        }
      }
    } catch (e) {
      verboseLog(`Failed to save cookies: ${e.message}`);
    }

    log(`Session ${sessionNum}: Completed (${sectionsToVisit.length} pages, visitor: ${visitorUuid?.substring(0, 8) || 'unknown'}...)`);

  } catch (error) {
    log(`Session ${sessionNum}: Error - ${error.message}`);
  } finally {
    // Close context with timeout to prevent hangs
    await withTimeout(context.close(), 5000);
  }
}

/**
 * Main function
 */
async function main() {
  console.log('');
  log('='.repeat(60));
  log('Headless Browser Traffic Generator for Rayne RUM');
  log('='.repeat(60));
  log(`Frontend URL:    ${config.frontendUrl}`);
  log(`Backend URL:     ${config.backendUrl}`);
  log(`Sessions:        ${config.continuous ? 'Continuous' : config.sessions}`);
  log(`Concurrent:      ${config.concurrent}`);
  log(`New user rate:   ${config.newUserRate}%`);
  log(`Headless mode:   ${config.headless}`);
  log(`Visitor pool:    ${config.poolFile}`);
  log('');

  // Load visitor pool
  loadPool();

  // Test connectivity
  log('Testing connectivity...');
  let testBrowser;
  try {
    testBrowser = await puppeteer.launch({
      headless: config.headless ? 'new' : false,
      args: [
        '--no-sandbox',
        '--disable-setuid-sandbox',
        '--disable-dev-shm-usage',
        '--disable-blink-features=AutomationControlled',
        '--disable-features=IsolateOrigins,site-per-process',
        '--disable-infobars',
        '--window-size=1920,1080',
      ],
      ignoreDefaultArgs: ['--enable-automation'],
    });
    const testPage = await testBrowser.newPage();
    await testPage.goto(config.frontendUrl, { timeout: 10000 });
    await testBrowser.close();
    log('Frontend connection successful!');
  } catch (error) {
    if (testBrowser) await testBrowser.close();
    log(`ERROR: Cannot connect to frontend at ${config.frontendUrl}`);
    log(`Make sure the frontend is running: ${error.message}`);
    process.exit(1);
  }

  log('');
  log('Starting traffic generation...');
  log('');

  const browser = await puppeteer.launch({
    headless: config.headless ? 'new' : false,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-blink-features=AutomationControlled',
      '--disable-features=IsolateOrigins,site-per-process',
      '--disable-infobars',
      '--window-size=1920,1080',
    ],
    ignoreDefaultArgs: ['--enable-automation'],
  });

  let completedSessions = 0;
  const startTime = Date.now();

  // Handle graceful shutdown
  let stopping = false;
  process.on('SIGINT', () => {
    if (stopping) process.exit(1);
    stopping = true;
    log('');
    log('Stopping... (press Ctrl+C again to force quit)');
  });

  const SESSION_TIMEOUT = 60000; // 60 second max per session

  if (config.continuous) {
    // Continuous mode
    let sessionNum = 0;
    const workers = [];

    for (let w = 0; w < config.concurrent; w++) {
      workers.push((async () => {
        while (!stopping) {
          sessionNum++;
          const currentSession = sessionNum;

          // Wrap session in timeout to prevent hangs
          const result = await withTimeout(
            simulateSession(browser, currentSession),
            SESSION_TIMEOUT,
            'timeout'
          );

          if (result === 'timeout') {
            log(`Session ${currentSession}: Timed out after ${SESSION_TIMEOUT/1000}s`);
          }
          completedSessions++;

          const delay = randomInt(config.minDelay, config.maxDelay);
          await sleep(delay);
        }
      })());
    }

    await Promise.all(workers);
  } else {
    // Fixed number of sessions
    const queue = [];
    for (let i = 1; i <= config.sessions; i++) {
      queue.push(i);
    }

    const workers = [];
    for (let w = 0; w < config.concurrent; w++) {
      workers.push((async () => {
        while (queue.length > 0 && !stopping) {
          const sessionNum = queue.shift();
          if (sessionNum) {
            // Wrap session in timeout to prevent hangs
            const result = await withTimeout(
              simulateSession(browser, sessionNum),
              SESSION_TIMEOUT,
              'timeout'
            );

            if (result === 'timeout') {
              log(`Session ${sessionNum}: Timed out after ${SESSION_TIMEOUT/1000}s`);
            }
            completedSessions++;

            if (queue.length > 0) {
              const delay = randomInt(config.minDelay, config.maxDelay);
              await sleep(delay);
            }
          }
        }
      })());
    }

    await Promise.all(workers);
  }

  await browser.close();

  const duration = ((Date.now() - startTime) / 1000).toFixed(1);
  log('');
  log('='.repeat(60));
  log(`Completed ${completedSessions} sessions in ${duration}s`);
  log(`Visitor pool size: ${Object.keys(visitorPool).length}`);
  log('');
  log('Check Datadog RUM Sessions:');
  log('  https://app.datadoghq.com/rum/sessions?query=%40service%3Arayne-frontend');
  log('='.repeat(60));
}

main().catch(error => {
  console.error('Fatal error:', error);
  process.exit(1);
});
