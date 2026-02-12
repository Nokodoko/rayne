#!/bin/bash
# Start the Monty chatbot gateway on base
# Uses Ollama with llama3.2:latest for lightweight, fast responses

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GATEWAY_PORT="${GATEWAY_PORT:-8001}"
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
OLLAMA_MODEL="${OLLAMA_MODEL:-llama3.2:latest}"
VENV_DIR="${SCRIPT_DIR}/.venv"
LOG_FILE="/tmp/base-gateway.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
err() { echo -e "${RED}[ERROR]${NC} $1" >&2; exit 1; }

# Check Ollama is running
if ! curl -sf http://localhost:11434/api/tags > /dev/null 2>&1; then
    info "Starting Ollama..."
    ollama serve > /tmp/ollama.log 2>&1 &
    sleep 3
fi

# Check model is available
if ! curl -sf http://localhost:11434/api/tags | grep -q "$OLLAMA_MODEL"; then
    info "Pulling $OLLAMA_MODEL..."
    ollama pull "$OLLAMA_MODEL"
fi

# Create/update venv
if [ ! -d "$VENV_DIR" ]; then
    info "Creating virtual environment..."
    python3 -m venv "$VENV_DIR"
fi

info "Installing dependencies..."
"$VENV_DIR/bin/pip" install -q -r "$SCRIPT_DIR/requirements.txt"

# Source Datadog LLM Observability environment if available
if [ -f "$HOME/.datadog-llm-env" ]; then
    . "$HOME/.datadog-llm-env"
fi

# Export env vars
export GATEWAY_PORT OLLAMA_HOST OLLAMA_MODEL

info "Starting gateway on port $GATEWAY_PORT..."
exec "$VENV_DIR/bin/python" -m uvicorn main:app --host 0.0.0.0 --port "$GATEWAY_PORT" --app-dir "$SCRIPT_DIR"
