#!/usr/bin/env bash
# deploy-llm-monitoring.sh
#
# Deploy Datadog LLM Observability instrumentation to the monty host.
#
# This script:
#   1. Installs ddtrace on the monty host via pip
#   2. Copies the LLM wrapper module to the monty project
#   3. Sets the required environment variables
#   4. Provides instructions for wrapping the monty gateway startup
#
# Prerequisites:
#   - SSH access to monty (192.168.50.68) as current user
#   - Python/pip available on monty
#   - Datadog agent reachable from monty at 192.168.50.179:8126

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
MONTY_HOST="192.168.50.68"
MONTY_USER="${MONTY_USER:-$(whoami)}"
MONTY_PROJECT_DIR="${MONTY_PROJECT_DIR:-/home/${MONTY_USER}/monty}"
MONTY_SSH="${MONTY_USER}@${MONTY_HOST}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRAPPER_FILE="${SCRIPT_DIR}/ddtrace-llm-wrapper.py"

# Datadog environment variables for LLM Observability
DD_LLMOBS_ENABLED="1"
DD_LLMOBS_ML_APP="monty-chatbot"
DD_LLMOBS_AGENTLESS_ENABLED="0"
DD_SERVICE="monty-llm"
DD_ENV="production"
DD_AGENT_HOST="192.168.50.179"
DD_TRACE_AGENT_PORT="8126"

# ---------------------------------------------------------------------------
# Helper functions
# ---------------------------------------------------------------------------
info()  { echo -e "\033[1;34m[INFO]\033[0m  $*"; }
ok()    { echo -e "\033[1;32m[OK]\033[0m    $*"; }
warn()  { echo -e "\033[1;33m[WARN]\033[0m  $*"; }
err()   { echo -e "\033[1;31m[ERROR]\033[0m $*"; exit 1; }

# ---------------------------------------------------------------------------
# DD_AGENT_HOST reachability through minikube
# ---------------------------------------------------------------------------
# If the Datadog agent runs inside minikube and monty is external, the agent's
# APM port (8126) must be reachable from the monty host. Common approaches:
#
#   Option A: minikube tunnel
#     Run `minikube tunnel` in a separate terminal. This exposes NodePort and
#     LoadBalancer services on the host network.
#
#   Option B: kubectl port-forward
#     kubectl port-forward svc/datadog-agent 8126:8126 --address 0.0.0.0
#     This forwards traffic from the minikube host's 8126 to the agent pod.
#
#   Option C: hostPort (current config)
#     The helm values set hostPort: 8126 on the trace-agent container, which
#     binds the port on the minikube node itself. Combined with
#     DD_APM_NON_LOCAL_TRAFFIC=true this allows external hosts to connect
#     directly to the minikube VM IP.
#
# Whichever method you choose, ensure monty can reach DD_AGENT_HOST:8126.
# ---------------------------------------------------------------------------

# ---------------------------------------------------------------------------
# Pre-flight: Check DD agent reachability from this machine
# ---------------------------------------------------------------------------
info "Pre-flight: checking if Datadog agent is reachable at ${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}..."
if curl -sf --max-time 5 -o /dev/null "http://${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}/info" 2>/dev/null; then
    ok "Datadog agent is reachable from this host."
else
    warn "Cannot reach Datadog agent at ${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT} from this host."
    warn "If the agent runs in minikube, you may need one of:"
    warn "  - minikube tunnel"
    warn "  - kubectl port-forward svc/datadog-agent 8126:8126 --address 0.0.0.0"
    warn "Continuing deployment anyway..."
fi

# ---------------------------------------------------------------------------
# Step 1: Install ddtrace on monty
# ---------------------------------------------------------------------------
# Note: If monty uses a virtualenv, activate it first or set MONTY_PROJECT_DIR
# to the venv root. If installing globally on a system-managed Python, you may
# need to add --break-system-packages to the pip command.
# ---------------------------------------------------------------------------
info "Installing ddtrace on ${MONTY_SSH}..."
ssh "${MONTY_SSH}" "pip install --upgrade ddtrace 2>&1" || {
    warn "pip install failed, trying pip3..."
    ssh "${MONTY_SSH}" "pip3 install --upgrade ddtrace 2>&1" || err "Failed to install ddtrace on monty."
}
ok "ddtrace installed on monty."

# ---------------------------------------------------------------------------
# Step 2: Copy the LLM wrapper module to monty
# ---------------------------------------------------------------------------
info "Copying LLM wrapper module to ${MONTY_SSH}:${MONTY_PROJECT_DIR}/..."
if [ ! -f "${WRAPPER_FILE}" ]; then
    err "Wrapper file not found: ${WRAPPER_FILE}"
fi

scp "${WRAPPER_FILE}" "${MONTY_SSH}:${MONTY_PROJECT_DIR}/ddtrace_llm_wrapper.py" || \
    err "Failed to copy wrapper to monty."

ok "LLM wrapper module deployed to ${MONTY_PROJECT_DIR}/ddtrace_llm_wrapper.py"

# ---------------------------------------------------------------------------
# Step 3: Create an environment file on monty
# ---------------------------------------------------------------------------
info "Creating Datadog LLM environment file on monty..."

ssh "${MONTY_SSH}" bash -s <<'REMOTE_SCRIPT'
cat > "${HOME}/.datadog-llm-env" <<'ENVFILE'
# Datadog LLM Observability environment variables
# Source this file before starting the monty gateway:
#   source ~/.datadog-llm-env

export DD_LLMOBS_ENABLED=1
export DD_LLMOBS_ML_APP=monty-chatbot
export DD_LLMOBS_AGENTLESS_ENABLED=0
export DD_SERVICE=monty-llm
export DD_ENV=production
export DD_AGENT_HOST=192.168.50.179
export DD_TRACE_AGENT_PORT=8126

# Optional: increase log level for debugging traces
# export DD_TRACE_DEBUG=true
# export DD_TRACE_LOG_FILE=/tmp/ddtrace-monty.log
ENVFILE
chmod 600 "${HOME}/.datadog-llm-env"
REMOTE_SCRIPT

ok "Environment file created at ~/.datadog-llm-env on monty."

# ---------------------------------------------------------------------------
# Step 4: Verify connectivity to the Datadog agent
# ---------------------------------------------------------------------------
info "Verifying monty can reach Datadog agent at ${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}..."

ssh "${MONTY_SSH}" "curl -sf -o /dev/null -w '%{http_code}' http://${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}/info 2>/dev/null" && \
    ok "Datadog agent is reachable from monty." || \
    warn "Could not reach Datadog agent at ${DD_AGENT_HOST}:${DD_TRACE_AGENT_PORT}. Ensure the agent is running and APM non-local traffic is enabled."

# ---------------------------------------------------------------------------
# Step 5: Print startup instructions
# ---------------------------------------------------------------------------
echo ""
echo "============================================================================="
echo "  Datadog LLM Observability Deployment Complete"
echo "============================================================================="
echo ""
echo "To start the monty gateway with LLM tracing, use one of these methods:"
echo ""
echo "  Method 1: ddtrace-run (recommended)"
echo "  -----------------------------------"
echo "  SSH into monty and run:"
echo ""
echo "    source ~/.datadog-llm-env"
echo "    cd ${MONTY_PROJECT_DIR}"
echo "    ddtrace-run python -m interfaces.api.server"
echo ""
echo "  Method 2: Programmatic instrumentation"
echo "  ---------------------------------------"
echo "  Add this to the top of your server startup code (before any LLM calls):"
echo ""
echo "    from ddtrace_llm_wrapper import enable_llm_monitoring, instrument_ollama_client"
echo "    enable_llm_monitoring()"
echo ""
echo "  Then instrument the OllamaClient in the lifespan handler:"
echo ""
echo "    ollama_client = OllamaClient(host=settings.ollama_host)"
echo "    instrument_ollama_client(ollama_client)"
echo ""
echo "  Method 3: systemd service (if monty runs as a service)"
echo "  ------------------------------------------------------"
echo "  Add the environment variables to your service unit file:"
echo ""
echo "    [Service]"
echo "    EnvironmentFile=/home/${MONTY_USER}/.datadog-llm-env"
echo "    ExecStart=/usr/bin/ddtrace-run python -m interfaces.api.server"
echo ""
echo "============================================================================="
echo ""
echo "Traces will appear in Datadog under:"
echo "  - APM > Services > monty-llm"
echo "  - LLM Observability > monty-chatbot"
echo ""
echo "============================================================================="
