#!/usr/bin/env python3
"""
Local notification server that receives webhooks from Rayne (in K8s)
and sends desktop notifications via notify-send.
"""

import argparse
import subprocess
from http.server import HTTPServer, BaseHTTPRequestHandler
import json


def parse_args():
    parser = argparse.ArgumentParser(
        prog='notify-server',
        description='Local notification server that receives webhooks from Rayne and sends desktop notifications via notify-send.',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
    %(prog)s                  Start server on default port 9999
    %(prog)s -p 8080          Start server on port 8080
    %(prog)s --bind 127.0.0.1 Bind to localhost only

Endpoints:
    POST /    Receive webhook payload and send desktop notification
    GET  /    Health check endpoint

Webhook Payload (Datadog webhook format):
    {
        "ALERT_STATE": "$ALERT_TRANSITION",
        "ALERT_TITLE": "$ALERT_TITLE",
        "APPLICATION_LONGNAME": "$TAGS[longname]",
        "APPLICATION_TEAM": "$TAGS[application_team]",
        "DETAILED_DESCRIPTION": "$EVENT_MSG",
        "IMPACT": "3-Moderate/Limited",
        "METRIC": "$ALERT_STATUS",
        "SUPPORT_GROUP": "$TAGS[support_group]",
        "THRESHOLD": "$THRESHOLD",
        "VALUE": "$VALUE",
        "URGENCY": "$ALERT_PRIORITY"
    }

Note: Requires 'notify-send' to be installed (libnotify-bin on Debian/Ubuntu).
"""
    )
    parser.add_argument(
        '-p', '--port',
        type=int,
        default=9999,
        help='Port to listen on (default: 9999)'
    )
    parser.add_argument(
        '-b', '--bind',
        type=str,
        default='0.0.0.0',
        help='Address to bind to (default: 0.0.0.0)'
    )
    return parser.parse_args()


args = parse_args()
PORT = args.port
BIND = args.bind

class NotifyHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)

        try:
            data = json.loads(body) if body else {}
        except json.JSONDecodeError:
            data = {}

        # Extract notification details - simple format from Rayne
        title = data.get('title', 'Datadog Webhook')
        message = data.get('message', '')
        urgency = data.get('urgency', 'critical')

        # Validate urgency
        if urgency not in ('critical', 'normal', 'low'):
            urgency = 'critical'

        notification_body = message if message else title

        # Log received webhook
        print(f"\n{'='*50}")
        print(f"[WEBHOOK] Received:")
        print(json.dumps(data, indent=2))

        # Send desktop notification with dunst hints for orange border
        # Requires dunst config rule for appname=Rayne with frame_color=#ff8c00
        try:
            subprocess.run([
                'notify-send',
                '-u', urgency,
                '-a', 'Rayne',
                '-h', 'string:x-dunst-stack-tag:rayne',
                '-h', 'string:fgcolor:#ffffff',
                '-h', 'string:bgcolor:#1a1a1a',
                '-h', 'string:frcolor:#ff8c00',
                title,
                notification_body
            ], check=True)
            print(f"[NOTIFY] Sent: {title} (orange border)")

            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({
                'status': 'ok',
                'message': 'Notification sent'
            }).encode())

        except subprocess.CalledProcessError as e:
            print(f"[ERROR] notify-send failed: {e}")
            self.send_response(500)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({
                'status': 'error',
                'message': str(e)
            }).encode())
        except FileNotFoundError:
            print("[ERROR] notify-send not found")
            self.send_response(500)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            self.wfile.write(json.dumps({
                'status': 'error',
                'message': 'notify-send not installed'
            }).encode())

    def do_GET(self):
        """Health check endpoint"""
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps({
            'status': 'ok',
            'service': 'notify-server'
        }).encode())

    def log_message(self, format, *args):
        """Suppress default logging"""
        pass


def main():
    server = HTTPServer((BIND, PORT), NotifyHandler)
    print(f"""
╔══════════════════════════════════════════════════════╗
║          Rayne Notification Server                   ║
╠══════════════════════════════════════════════════════╣
║  Listening on: http://{BIND}:{PORT:<5}                 ║
║                                                      ║
║  From Kubernetes, Rayne can reach this at:           ║
║    http://host.minikube.internal:{PORT:<5}              ║
║                                                      ║
║  POST /  - Receive webhook, send notification        ║
║  GET  /  - Health check                              ║
║                                                      ║
║  Run with -h or --help for usage information         ║
╚══════════════════════════════════════════════════════╝
    """)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == '__main__':
    main()
