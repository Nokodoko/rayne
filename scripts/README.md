# Rayne Scripts

## notify-server.py

A local notification server that receives webhooks from Rayne (running in Kubernetes) and sends desktop notifications via `notify-send`.

### Architecture

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Datadog       │────▶│   Rayne      │────▶│  notify-server  │
│   Webhook       │     │   (K8s)      │     │  (localhost)    │
└─────────────────┘     └──────────────┘     └─────────────────┘
                                                     │
                                                     ▼
                                              ┌─────────────┐
                                              │ notify-send │
                                              │ (desktop)   │
                                              └─────────────┘
```

### Requirements

- Python 3.6+
- `notify-send` (part of `libnotify` package)

```bash
# Arch Linux
sudo pacman -S libnotify

# Ubuntu/Debian
sudo apt install libnotify-bin

# Fedora
sudo dnf install libnotify
```

### Usage

```bash
# Start with default port (9999)
./notify-server.py

# Start with custom port
./notify-server.py 8888
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Health check |
| POST | `/` | Receive webhook and send notification |

### Webhook Payload

```json
{
  "title": "Alert Title",
  "message": "Alert message body",
  "urgency": "critical"
}
```

**Urgency levels:** `low`, `normal`, `critical`

### Testing

```bash
# Health check
curl http://localhost:9999/

# Send test notification
curl -X POST http://localhost:9999/ \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Alert", "message": "This is a test notification", "urgency": "critical"}'
```

### Kubernetes Integration

From inside minikube, Rayne reaches the local server via:
```
http://host.minikube.internal:9999
```

Configure via environment variable in Rayne:
```yaml
- name: NOTIFY_SERVER_URL
  value: "http://host.minikube.internal:9999"
```

---

## Expanding the Server

### Adding New Notification Backends

The server can be extended to support multiple notification methods. Here's how:

#### 1. Create a Notification Handler Base

```python
# notifications/base.py
from abc import ABC, abstractmethod

class NotificationHandler(ABC):
    @abstractmethod
    def send(self, title: str, message: str, urgency: str = "normal") -> bool:
        """Send a notification. Returns True on success."""
        pass
```

#### 2. Implement Different Backends

**Slack Notifications:**
```python
# notifications/slack.py
import requests
from .base import NotificationHandler

class SlackHandler(NotificationHandler):
    def __init__(self, webhook_url: str):
        self.webhook_url = webhook_url

    def send(self, title: str, message: str, urgency: str = "normal") -> bool:
        color = {"critical": "danger", "normal": "warning", "low": "good"}.get(urgency, "warning")
        payload = {
            "attachments": [{
                "color": color,
                "title": title,
                "text": message
            }]
        }
        resp = requests.post(self.webhook_url, json=payload)
        return resp.status_code == 200
```

**Discord Notifications:**
```python
# notifications/discord.py
import requests
from .base import NotificationHandler

class DiscordHandler(NotificationHandler):
    def __init__(self, webhook_url: str):
        self.webhook_url = webhook_url

    def send(self, title: str, message: str, urgency: str = "normal") -> bool:
        color = {"critical": 0xFF0000, "normal": 0xFFA500, "low": 0x00FF00}.get(urgency, 0xFFA500)
        payload = {
            "embeds": [{
                "title": title,
                "description": message,
                "color": color
            }]
        }
        resp = requests.post(self.webhook_url, json=payload)
        return resp.status_code == 204
```

**Email Notifications:**
```python
# notifications/email.py
import smtplib
from email.mime.text import MIMEText
from .base import NotificationHandler

class EmailHandler(NotificationHandler):
    def __init__(self, smtp_host: str, smtp_port: int, username: str, password: str, to_addr: str):
        self.smtp_host = smtp_host
        self.smtp_port = smtp_port
        self.username = username
        self.password = password
        self.to_addr = to_addr

    def send(self, title: str, message: str, urgency: str = "normal") -> bool:
        msg = MIMEText(message)
        msg['Subject'] = f"[{urgency.upper()}] {title}"
        msg['From'] = self.username
        msg['To'] = self.to_addr

        try:
            with smtplib.SMTP(self.smtp_host, self.smtp_port) as server:
                server.starttls()
                server.login(self.username, self.password)
                server.send_message(msg)
            return True
        except Exception:
            return False
```

**PagerDuty Notifications:**
```python
# notifications/pagerduty.py
import requests
from .base import NotificationHandler

class PagerDutyHandler(NotificationHandler):
    def __init__(self, routing_key: str):
        self.routing_key = routing_key
        self.url = "https://events.pagerduty.com/v2/enqueue"

    def send(self, title: str, message: str, urgency: str = "normal") -> bool:
        severity = {"critical": "critical", "normal": "warning", "low": "info"}.get(urgency, "warning")
        payload = {
            "routing_key": self.routing_key,
            "event_action": "trigger",
            "payload": {
                "summary": title,
                "severity": severity,
                "source": "rayne",
                "custom_details": {"message": message}
            }
        }
        resp = requests.post(self.url, json=payload)
        return resp.status_code == 202
```

#### 3. Update the Server to Use Multiple Handlers

```python
#!/usr/bin/env python3
"""Enhanced notification server with multiple backends."""

import json
import os
import subprocess
from http.server import HTTPServer, BaseHTTPRequestHandler

# Configuration from environment
SLACK_WEBHOOK = os.getenv("SLACK_WEBHOOK_URL")
DISCORD_WEBHOOK = os.getenv("DISCORD_WEBHOOK_URL")
PAGERDUTY_KEY = os.getenv("PAGERDUTY_ROUTING_KEY")

class NotificationManager:
    def __init__(self):
        self.handlers = []

        # Always add desktop notifications
        self.handlers.append(("desktop", self._send_desktop))

        # Add optional backends
        if SLACK_WEBHOOK:
            self.handlers.append(("slack", self._send_slack))
        if DISCORD_WEBHOOK:
            self.handlers.append(("discord", self._send_discord))
        if PAGERDUTY_KEY:
            self.handlers.append(("pagerduty", self._send_pagerduty))

    def _send_desktop(self, title, message, urgency):
        subprocess.run(['notify-send', '-u', urgency, '-a', 'Rayne', title, message])
        return True

    def _send_slack(self, title, message, urgency):
        # Implementation from above
        pass

    def _send_discord(self, title, message, urgency):
        # Implementation from above
        pass

    def _send_pagerduty(self, title, message, urgency):
        # Implementation from above
        pass

    def notify_all(self, title: str, message: str, urgency: str = "normal"):
        results = {}
        for name, handler in self.handlers:
            try:
                results[name] = handler(title, message, urgency)
            except Exception as e:
                results[name] = f"error: {e}"
        return results

manager = NotificationManager()

class NotifyHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)
        data = json.loads(body) if body else {}

        title = data.get('title', 'Datadog Webhook')
        message = data.get('message', 'No message')
        urgency = data.get('urgency', 'critical')

        results = manager.notify_all(title, message, urgency)

        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps({"status": "ok", "results": results}).encode())
```

### Adding Filtering and Routing

Route notifications based on tags or severity:

```python
class NotificationRouter:
    def __init__(self):
        self.rules = []

    def add_rule(self, condition, handlers):
        """
        condition: callable that takes (title, message, urgency, tags) -> bool
        handlers: list of handler names to notify when condition matches
        """
        self.rules.append((condition, handlers))

    def route(self, title, message, urgency, tags=None):
        """Returns list of handler names to use for this notification."""
        tags = tags or []
        matched_handlers = set()

        for condition, handlers in self.rules:
            if condition(title, message, urgency, tags):
                matched_handlers.update(handlers)

        # Default to desktop if no rules match
        return list(matched_handlers) if matched_handlers else ["desktop"]

# Example usage
router = NotificationRouter()

# Critical alerts go everywhere
router.add_rule(
    lambda t, m, u, tags: u == "critical",
    ["desktop", "slack", "pagerduty"]
)

# Production alerts go to Slack
router.add_rule(
    lambda t, m, u, tags: "env:prod" in tags,
    ["slack"]
)

# Database alerts go to specific channel
router.add_rule(
    lambda t, m, u, tags: any("service:postgres" in tag or "service:mysql" in tag for tag in tags),
    ["slack", "pagerduty"]
)
```

### Adding Persistent Storage

Store notification history for debugging:

```python
import sqlite3
from datetime import datetime

class NotificationStore:
    def __init__(self, db_path="notifications.db"):
        self.conn = sqlite3.connect(db_path, check_same_thread=False)
        self._init_db()

    def _init_db(self):
        self.conn.execute("""
            CREATE TABLE IF NOT EXISTS notifications (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                title TEXT,
                message TEXT,
                urgency TEXT,
                handlers TEXT,
                results TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
            )
        """)
        self.conn.commit()

    def store(self, title, message, urgency, handlers, results):
        self.conn.execute(
            "INSERT INTO notifications (title, message, urgency, handlers, results) VALUES (?, ?, ?, ?, ?)",
            (title, message, urgency, json.dumps(handlers), json.dumps(results))
        )
        self.conn.commit()

    def get_recent(self, limit=50):
        cursor = self.conn.execute(
            "SELECT * FROM notifications ORDER BY created_at DESC LIMIT ?",
            (limit,)
        )
        return cursor.fetchall()
```

### Running as a Systemd Service

Create `/etc/systemd/system/rayne-notify.service`:

```ini
[Unit]
Description=Rayne Notification Server
After=network.target

[Service]
Type=simple
User=your-username
WorkingDirectory=/home/your-username/Portfolio/rayne/scripts
ExecStart=/usr/bin/python3 notify-server.py
Restart=always
RestartSec=5

# Optional: Add environment variables for backends
Environment="SLACK_WEBHOOK_URL=https://hooks.slack.com/services/xxx"
Environment="DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/xxx"

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl enable rayne-notify
sudo systemctl start rayne-notify

# Check status
sudo systemctl status rayne-notify

# View logs
journalctl -u rayne-notify -f
```

---

## traffic-generator.sh

Generates realistic API traffic for Rayne APM demos.

### Usage

```bash
# Start with default localhost:8080
./traffic-generator.sh start

# Start with custom URL (e.g., minikube)
./traffic-generator.sh start http://192.168.49.2:30821

# With custom failure rate
FAILURE_RATE=20 ./traffic-generator.sh start

# Stop
./traffic-generator.sh stop

# Check status
./traffic-generator.sh status
```

### What It Does

- Sends continuous traffic to all Rayne API endpoints
- Injects configurable error rate (default 10%)
- Logs activity to `/tmp/rayne-traffic-generator.log`

### Monitoring

```bash
# Watch traffic generator logs
tail -f /tmp/rayne-traffic-generator.log

# Check if running
./traffic-generator.sh status
```
