#!/bin/bash
# Install the monty-gateway systemd service
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVICE_FILE="$SCRIPT_DIR/monty-gateway.service"

echo "Installing monty-gateway.service..."
sudo cp "$SERVICE_FILE" /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable monty-gateway
echo "Service installed. Start with: sudo systemctl start monty-gateway"
echo "Check status: sudo systemctl status monty-gateway"
echo "View logs: journalctl -u monty-gateway -f"
