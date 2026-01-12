#!/usr/bin/env python3
"""
Local notification server that receives webhooks from Rayne (in K8s)
and sends desktop notifications via notify-send.

Usage:
    ./notify-server.py [port]

Default port: 9999
"""

import subprocess
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
import json

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 9999

class NotifyHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length)

        try:
            data = json.loads(body) if body else {}
        except json.JSONDecodeError:
            data = {}

        # Extract notification details
        title = data.get('title', 'Datadog Webhook')
        message = data.get('message', data.get('alert_status', 'No message'))
        urgency = data.get('urgency', 'critical')

        # Log received webhook
        print(f"\n{'='*50}")
        print(f"[WEBHOOK] Received:")
        print(json.dumps(data, indent=2))

        # Send desktop notification
        try:
            subprocess.run([
                'notify-send',
                '-u', urgency,
                '-a', 'Rayne',
                title,
                message
            ], check=True)
            print(f"[NOTIFY] Sent: {title} - {message}")

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
    server = HTTPServer(('0.0.0.0', PORT), NotifyHandler)
    print(f"""
╔══════════════════════════════════════════════════════╗
║          Rayne Notification Server                   ║
╠══════════════════════════════════════════════════════╣
║  Listening on: http://0.0.0.0:{PORT:<5}                 ║
║                                                      ║
║  From Kubernetes, Rayne can reach this at:           ║
║    http://host.minikube.internal:{PORT:<5}              ║
║                                                      ║
║  POST /  - Receive webhook, send notification        ║
║  GET  /  - Health check                              ║
╚══════════════════════════════════════════════════════╝
    """)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()


if __name__ == '__main__':
    main()
