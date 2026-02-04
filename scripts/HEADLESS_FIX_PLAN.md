# Headless Traffic Generator Fix

## Problem Summary

Datadog RUM SDK v6 includes bot detection that filters out headless browser traffic. The SDK checks for common automation signatures like `navigator.webdriver`, missing plugins, and Chrome automation flags.

Additionally, the SDK batches events and only flushes on page unload. Puppeteer closing contexts does not trigger unload events, so batched RUM data never reaches Datadog.

## Solution Overview

1. **Stealth Plugin**: Use puppeteer-extra with stealth plugin to evade bot detection
2. **Launch Flags**: Add `--disable-blink-features=AutomationControlled` to hide automation
3. **Fingerprint Overrides**: Override `navigator.webdriver` and other detectable properties
4. **Forced Flush**: Trigger SDK flush before closing browser context

## Changes Made

### Dependencies

```bash
npm install puppeteer-extra puppeteer-extra-plugin-stealth
```

### Import Updates

```javascript
const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');
puppeteer.use(StealthPlugin());
```

### Launch Arguments

```javascript
const browser = await puppeteer.launch({
  args: [
    '--disable-blink-features=AutomationControlled',
    '--no-sandbox',
    '--disable-setuid-sandbox'
  ]
});
```

### Stealth Evasions

Added `applyStealthEvasions()` function that overrides:
- `navigator.webdriver` (set to undefined)
- `navigator.plugins` (fake plugin array)
- `navigator.languages` (realistic language list)
- Chrome runtime properties

### SDK Flush Before Close

```javascript
await page.evaluate(() => {
  if (window.DD_RUM && typeof window.DD_RUM.getInternalContext === 'function') {
    // Trigger flush by dispatching unload-like events
    window.dispatchEvent(new Event('beforeunload'));
    window.dispatchEvent(new Event('pagehide'));
  }
});
await page.waitForTimeout(2000); // Allow flush to complete
```

## Verification Steps

1. **Run with visible browser**:
   ```bash
   node headless-traffic-generator.js --verbose --no-headless --sessions 3
   ```

2. **Check console for `[DD Request]` logs** confirming SDK sends data to Datadog intake

3. **Wait 2-5 minutes** for Datadog processing

4. **Check Datadog RUM sessions**:
   https://app.datadoghq.com/rum/sessions

## Usage

```bash
# Basic usage
node headless-traffic-generator.js --sessions 5

# Debugging mode (visible browser + verbose logs)
node headless-traffic-generator.js --verbose --no-headless --sessions 3

# Using npm scripts
npm run traffic           # headless mode
npm run traffic:visible   # visible browser for debugging
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| No `[DD Request]` logs | Check SDK initialization, verify RUM credentials |
| Sessions not appearing in Datadog | Wait longer (up to 5 min), check for bot filtering |
| Browser crashes | Reduce concurrent sessions, add more memory |
| Stealth not working | Update stealth plugin, check for new detection methods |
