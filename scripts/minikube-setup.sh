#!/usr/bin/env bash

# Rayne Minikube Setup Script
# This script sets up the Rayne application in a local minikube cluster

set -e

#=============================================================================
# COLORS & STYLING
#=============================================================================
capColor() { gum style --foreground "#118DFF" "$1"; }
redColor() { gum style --foreground "#D82C20" "$1"; }
greenColor() { gum style --foreground "#00FF00" "$1"; }
purpleColor() { gum style --foreground "#9400D3" "$1"; }

#=============================================================================
# USAGE/HELP
#=============================================================================
show_usage() {
    cat << EOF
Rayne Minikube Setup Script

USAGE:
    $(basename "$0") [OPTIONS]

OPTIONS:
    -a, --api-key KEY      DD_API_KEY for sub-agent (incident reports)
    -p, --app-key KEY      DD_APP_KEY for sub-agent (incident reports)
    -s, --site SITE        Datadog site: 'com' (commercial) or 'gov' (government)
    -r, --rayne-keys MODE  Rayne service keys: 'default' or 'same' (as sub-agent)
    -h, --help             Show this help message

EXAMPLES:
    # Interactive mode (gum prompts)
    $(basename "$0")

    # Direct invocation with flags
    $(basename "$0") -a "your-api-key" -p "your-app-key" -s gov -r same

    # Commercial site with default Rayne keys
    $(basename "$0") -a "api-key" -p "app-key" -s com -r default

NOTES:
    - If no flags provided, interactive gum prompts will guide configuration
    - Sub-agent keys are used for dd_lib access and incident report creation
    - Rayne keys are used for APM, DBM, and log collection
    - 'default' Rayne keys use TF_VAR_ecco_dd_* environment variables
    - 'same' Rayne keys use the same keys as the sub-agent

EOF
}

#=============================================================================
# DEFAULTS
#=============================================================================
SUBAGENT_API_KEY=""
SUBAGENT_APP_KEY=""
SUBAGENT_SITE="datadoghq.com"
RAYNE_KEY_MODE="default"

#=============================================================================
# PARSE FLAGS
#=============================================================================
while [[ $# -gt 0 ]]; do
    case "$1" in
        -a|--api-key)
            SUBAGENT_API_KEY="$2"
            shift 2
            ;;
        -p|--app-key)
            SUBAGENT_APP_KEY="$2"
            shift 2
            ;;
        -s|--site)
            if [ "$2" = "gov" ]; then
                SUBAGENT_SITE="ddog-gov.com"
            else
                SUBAGENT_SITE="datadoghq.com"
            fi
            shift 2
            ;;
        -r|--rayne-keys)
            RAYNE_KEY_MODE="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

#=============================================================================
# INTERACTIVE MODE (GUM PROMPTS)
#=============================================================================
if [ -z "$SUBAGENT_API_KEY" ] || [ -z "$SUBAGENT_APP_KEY" ]; then
    # Check if gum is installed
    if ! command -v gum &> /dev/null; then
        echo "Error: gum is not installed and no flags provided"
        echo "Install gum: https://github.com/charmbracelet/gum"
        echo "Or use flags: $(basename "$0") -h"
        exit 1
    fi

    gum style \
        --border double \
        --padding "1" \
        --border-foreground "#9400D3" \
        "Rayne Minikube Setup"

    echo ""
    echo "$(purpleColor "Step 1:") Choose Datadog $(redColor "domain")"
    SITE_CHOICE=$(gum choose "Commercial (app.datadoghq.com)" "Government (app.ddog-gov.com)")
    if [[ "$SITE_CHOICE" == *"Government"* ]]; then
        SUBAGENT_SITE="ddog-gov.com"
    else
        SUBAGENT_SITE="datadoghq.com"
    fi
    echo "  Selected: $(greenColor "$SUBAGENT_SITE")"

    echo ""
    echo "$(purpleColor "Step 2:") Choose $(redColor "API Key") source for $(capColor "Sub-Agent")"

    # Discover environment variables containing DD_API_KEY and build list
    API_KEY_LIST=$(printf "Enter Manually\n"; env | grep -i "DD_API_KEY\|DD_API\|API_KEY" | grep -v "^_" | cut -d= -f1 | sort -u)

    API_KEY_CHOICE=$(echo "$API_KEY_LIST" | gum filter --placeholder "Search API key env vars...")
    if [ "$API_KEY_CHOICE" = "Enter Manually" ]; then
        SUBAGENT_API_KEY=$(gum input --placeholder "Enter DD_API_KEY" --password)
    else
        SUBAGENT_API_KEY="${!API_KEY_CHOICE}"
        echo "  Using: $(greenColor "$API_KEY_CHOICE")"
    fi

    echo ""
    echo "$(purpleColor "Step 3:") Choose $(redColor "APP Key") source for $(capColor "Sub-Agent")"

    # Discover environment variables containing DD_APP_KEY and build list
    APP_KEY_LIST=$(printf "Enter Manually\n"; env | grep -i "DD_APP_KEY\|DD_APP\|APP_KEY" | grep -v "^_" | cut -d= -f1 | sort -u)

    APP_KEY_CHOICE=$(echo "$APP_KEY_LIST" | gum filter --placeholder "Search APP key env vars...")
    if [ "$APP_KEY_CHOICE" = "Enter Manually" ]; then
        SUBAGENT_APP_KEY=$(gum input --placeholder "Enter DD_APP_KEY" --password)
    else
        SUBAGENT_APP_KEY="${!APP_KEY_CHOICE}"
        echo "  Using: $(greenColor "$APP_KEY_CHOICE")"
    fi

    echo ""
    echo "$(purpleColor "Step 4:") Choose $(redColor "Rayne Service") key configuration"
    RAYNE_CHOICE=$(gum choose "Default (use TF_VAR_ecco_dd_* keys)" "Same as Sub-Agent (use keys from above)")
    if [[ "$RAYNE_CHOICE" == *"Same"* ]]; then
        RAYNE_KEY_MODE="same"
    else
        RAYNE_KEY_MODE="default"
    fi
    echo "  Selected: $(greenColor "$RAYNE_KEY_MODE")"
    echo ""
fi

#=============================================================================
# SET RAYNE KEYS BASED ON MODE
#=============================================================================
if [ "$RAYNE_KEY_MODE" = "same" ]; then
    RAYNE_API_KEY="$SUBAGENT_API_KEY"
    RAYNE_APP_KEY="$SUBAGENT_APP_KEY"
else
    # Default mode - use TF_VAR_ecco_dd_* keys
    if [ -z "$TF_VAR_ecco_dd_api_key" ]; then
        echo "Error: TF_VAR_ecco_dd_api_key not set (required for 'default' Rayne key mode)"
        echo "Set it: export TF_VAR_ecco_dd_api_key=your-api-key"
        echo "Or use: $(basename "$0") -r same"
        exit 1
    fi
    if [ -z "$TF_VAR_ecco_dd_app_key" ]; then
        echo "Error: TF_VAR_ecco_dd_app_key not set (required for 'default' Rayne key mode)"
        echo "Set it: export TF_VAR_ecco_dd_app_key=your-app-key"
        echo "Or use: $(basename "$0") -r same"
        exit 1
    fi
    RAYNE_API_KEY="$TF_VAR_ecco_dd_api_key"
    RAYNE_APP_KEY="$TF_VAR_ecco_dd_app_key"
fi

#=============================================================================
# DISPLAY CONFIGURATION
#=============================================================================
echo "=== Rayne Minikube Setup ==="
echo ""
echo "Configuration:"
echo "  Sub-Agent Site:     $SUBAGENT_SITE"
echo "  Sub-Agent API Key:  ${SUBAGENT_API_KEY:0:8}..."
echo "  Sub-Agent APP Key:  ${SUBAGENT_APP_KEY:0:8}..."
echo "  Rayne Key Mode:     $RAYNE_KEY_MODE"
echo "  Rayne API Key:      ${RAYNE_API_KEY:0:8}..."
echo "  Rayne APP Key:      ${RAYNE_APP_KEY:0:8}..."
echo ""

# Check if minikube is installed
if ! command -v minikube &> /dev/null; then
    echo "Error: minikube is not installed. Please install minikube first."
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed. Please install kubectl first."
    exit 1
fi

# Start minikube if not running
if ! minikube status &> /dev/null; then
    echo "Starting minikube..."
    # Increased memory for Ollama + Qdrant + Rayne + PostgreSQL + Datadog Agent
    minikube start --driver=docker --cpus=4 --memory=12288
else
    echo "Minikube is already running"
    echo "Note: For full functionality, ensure minikube has at least 12GB RAM"
    echo "Restart with: minikube delete && minikube start --cpus=4 --memory=12288"
fi

# Build the Docker image locally using buildx
echo ""
echo "Building Rayne Docker image..."
cd "$(dirname "$0")/../mkii_ddog_server"
DOCKER_BUILDKIT=1 docker build -t rayne:latest .

# Build the Claude Agent sidecar image
echo ""
echo "Building Claude Agent Docker image..."
cd "$(dirname "$0")/.."
DOCKER_BUILDKIT=1 docker build -t claude-agent:latest -f docker/claude-agent/Dockerfile .

# Load images into minikube (works for both single and multi-node)
echo ""
echo "Loading images into minikube..."
minikube image load rayne:latest
minikube image load claude-agent:latest

export IMAGE_NAME="rayne:latest"

# Apply Kubernetes manifests
echo ""
echo "Applying Kubernetes manifests..."
cd "$(dirname "$0")/../k8s"

# Apply in order
kubectl apply -f postgres-deployment.yaml
echo "Waiting for PostgreSQL pod to be created..."
sleep 5
until kubectl get pods -l app=postgres 2>/dev/null | grep -q postgres; do
    echo "  Waiting for pod to appear..."
    sleep 2
done
echo "Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres --timeout=120s

# Create subagent-datadog-secrets (for Claude agent sidecar - incident reports)
echo "Creating Sub-Agent Datadog secrets..."
kubectl create secret generic subagent-datadog-secrets \
    --from-literal=api-key="$SUBAGENT_API_KEY" \
    --from-literal=app-key="$SUBAGENT_APP_KEY" \
    --from-literal=site="$SUBAGENT_SITE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create datadog-secrets (for Rayne service - APM, DBM, logs)
echo "Creating Rayne Datadog secrets..."
kubectl create secret generic datadog-secrets \
    --from-literal=api-key="$RAYNE_API_KEY" \
    --from-literal=app-key="$RAYNE_APP_KEY" \
    --dry-run=client -o yaml | kubectl apply -f -

# Create Anthropic secrets for Claude Agent
echo ""
echo "Creating Anthropic secrets..."
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Warning: ANTHROPIC_API_KEY not set. Claude Agent will not function."
    echo "Set it with: export ANTHROPIC_API_KEY=your-key"
    kubectl create secret generic anthropic-secrets \
        --from-literal=api-key="placeholder-key" \
        --dry-run=client -o yaml | kubectl apply -f -
else
    kubectl create secret generic anthropic-secrets \
        --from-literal=api-key="$ANTHROPIC_API_KEY" \
        --dry-run=client -o yaml | kubectl apply -f -
    echo "✓ Anthropic API key configured"
fi

# Apply assets ConfigMap
echo ""
echo "Applying assets ConfigMap..."
kubectl apply -f assets-configmap.yaml

# Deploy Qdrant vector DB
echo ""
echo "=== Deploying Qdrant Vector DB ==="
kubectl apply -f qdrant-deployment.yaml
echo "Waiting for Qdrant to be ready..."
kubectl wait --for=condition=ready pod -l app=qdrant --timeout=120s 2>/dev/null || \
    echo "  Note: Qdrant pod may still be starting..."

# Deploy Ollama for embeddings
echo ""
echo "=== Deploying Ollama (Gemma 2B) ==="
kubectl apply -f ollama-deployment.yaml
echo "Note: Ollama will download Gemma model on first start (~1.5GB)"
echo "This may take several minutes..."
kubectl wait --for=condition=ready pod -l app=ollama --timeout=300s 2>/dev/null || \
    echo "  Note: Ollama pod may still be downloading the model..."

# Install Datadog Agent FIRST (before Rayne) so the service is available
echo ""
echo "=== Installing Datadog Agent ==="

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo "Warning: helm is not installed. Skipping Datadog Agent installation."
    echo "To install helm: https://helm.sh/docs/intro/install/"
    echo "APM tracing will not work without the Datadog Agent."
else
    # Add Datadog Helm repository
    echo "Adding Datadog Helm repository..."
    helm repo add datadog https://helm.datadoghq.com 2>/dev/null || true
    helm repo update

    # Check if datadog-agent is already installed
    if helm list | grep -q datadog-agent; then
        echo "Upgrading existing Datadog Agent..."
        helm upgrade datadog-agent datadog/datadog \
            --set datadog.apiKey="$RAYNE_API_KEY" \
            --set datadog.appKey="$RAYNE_APP_KEY" \
            --set datadog.clusterName=rayne \
            --set datadog.site=datadoghq.com \
            --set agents.enabled=true \
            --set clusterAgent.enabled=true \
            -f "$(dirname "$0")/../helm/values.yaml"
    else
        echo "Installing Datadog Agent..."
        helm install datadog-agent datadog/datadog \
            --set datadog.apiKey="$RAYNE_API_KEY" \
            --set datadog.appKey="$RAYNE_APP_KEY" \
            --set datadog.clusterName=rayne \
            --set datadog.site=datadoghq.com \
            --set agents.enabled=true \
            --set clusterAgent.enabled=true \
            -f "$(dirname "$0")/../helm/values.yaml"
    fi

    echo "Waiting for Datadog Agent to be ready..."
    kubectl wait --for=condition=ready pod -l app=datadog-agent --timeout=180s 2>/dev/null || \
        echo "  Note: Datadog Agent pods may still be starting..."

    # Wait for the datadog-agent service to be available
    echo "Waiting for Datadog Agent service to be available..."
    until kubectl get svc datadog-agent 2>/dev/null | grep -q datadog-agent; do
        echo "  Waiting for datadog-agent service..."
        sleep 2
    done
    echo "✓ Datadog Agent service is available"

    # Wait for agent to actually respond on port 8126
    echo "Waiting for Datadog Agent APM endpoint to be ready..."
    AGENT_READY=false
    for i in $(seq 1 30); do
        if kubectl run curl-test --rm -i --restart=Never --image=curlimages/curl:latest -- \
            curl -s --connect-timeout 2 http://datadog-agent:8126/info > /dev/null 2>&1; then
            AGENT_READY=true
            break
        fi
        echo "  Waiting for APM endpoint (attempt $i/30)..."
        sleep 3
    done
    if [ "$AGENT_READY" = true ]; then
        echo "✓ Datadog Agent APM endpoint is ready"
    else
        echo "⚠ Warning: Agent APM endpoint may not be ready yet"
    fi
fi

# Now deploy Rayne (after Datadog Agent is ready)
echo ""
echo "=== Deploying Rayne ==="
kubectl apply -f rayne-deployment.yaml

echo "Waiting for Rayne pod to be created..."
sleep 5
until kubectl get pods -l app=rayne 2>/dev/null | grep -q rayne; do
    echo "  Waiting for pod to appear..."
    sleep 2
done
echo "Waiting for Rayne to be ready..."
kubectl wait --for=condition=ready pod -l app=rayne --timeout=120s

# Get service URL
echo ""
echo "=== Deployment Complete ==="
echo ""
RAYNE_URL=$(minikube service rayne-service --url 2>/dev/null || echo "http://$(minikube ip):$(kubectl get svc rayne-service -o jsonpath='{.spec.ports[0].nodePort}')")
echo "Rayne API: $RAYNE_URL"
echo ""
echo "=========================================="
echo "         Available API Endpoints          "
echo "=========================================="
echo ""
echo "Health & Auth:"
echo "  GET  /health                    - Health check"
echo "  POST /login                     - User login"
echo "  POST /register                  - User registration"
echo ""
echo "Downtimes & Events:"
echo "  GET  /v1/downtimes              - List downtimes"
echo "  GET  /v1/events                 - List events"
echo ""
echo "Hosts:"
echo "  GET  /v1/hosts                  - List all hosts"
echo "  GET  /v1/hosts/active           - Get active host count"
echo "  GET  /v1/hosts/tags             - Get all host tags"
echo "  GET  /v1/hosts/{hostname}/tags  - Get tags for specific host"
echo ""
echo "Monitors:"
echo "  GET  /v1/monitors               - List monitors (paginated)"
echo "  GET  /v1/monitors/triggered     - Get triggered monitors"
echo "  GET  /v1/monitors/ids           - Get monitor IDs and names"
echo "  GET  /v1/monitors/pages         - Get pagination metadata"
echo "  GET  /v1/monitors/{id}          - Get specific monitor"
echo ""
echo "Logs:"
echo "  POST /v1/logs/search            - Search logs (simple)"
echo "  POST /v1/logs/search/advanced   - Search logs (advanced)"
echo ""
echo "Service Catalog:"
echo "  GET  /v1/services               - List service definitions"
echo "  POST /v1/services/definitions   - Create service definition"
echo "  POST /v1/services/definitions/advanced - Create (full schema)"
echo ""
echo "Webhooks:"
echo "  POST /v1/webhooks/receive       - Receive webhook from Datadog"
echo "  GET  /v1/webhooks/events        - List stored webhook events"
echo "  GET  /v1/webhooks/events/{id}   - Get specific webhook event"
echo "  GET  /v1/webhooks/monitor/{id}  - Get events by monitor ID"
echo "  POST /v1/webhooks/create        - Create webhook in Datadog"
echo "  POST /v1/webhooks/config        - Save webhook config"
echo "  GET  /v1/webhooks/config        - Get webhook configs"
echo "  GET  /v1/webhooks/stats         - Get webhook statistics"
echo "  POST /v1/webhooks/reprocess     - Reprocess pending webhooks"
echo ""
echo "RUM (Real User Monitoring):"
echo "  POST /v1/rum/init               - Initialize visitor (get UUID)"
echo "  POST /v1/rum/track              - Track RUM event"
echo "  POST /v1/rum/session/end        - End session"
echo "  GET  /v1/rum/visitor/{uuid}     - Get visitor by UUID"
echo "  GET  /v1/rum/session/{id}       - Get session by ID"
echo "  GET  /v1/rum/visitors           - Get unique visitors"
echo "  GET  /v1/rum/analytics          - Get RUM analytics"
echo "  GET  /v1/rum/sessions           - Get recent sessions"
echo ""
echo "Demo Data:"
echo "  POST /v1/demo/seed/webhooks     - Seed webhook events"
echo "  POST /v1/demo/seed/rum          - Seed RUM data"
echo "  POST /v1/demo/seed/all          - Seed all demo data"
echo "  GET  /v1/demo/monitors          - Generate sample monitors"
echo "  GET  /v1/demo/status            - Get demo data status"
echo ""
echo "Private Locations:"
echo "  GET  /v1/pl/refresh/{name}      - Refresh private location"
echo ""
echo "Claude Agent (Internal Sidecar):"
echo "  POST /analyze                   - RCA analysis via Claude Code"
echo "  POST /generate-notebook         - Generate Datadog notebook"
echo "  GET  /templates                 - List available templates"
echo "  GET  /health                    - Claude Agent health check"
echo ""
echo "=========================================="
echo ""
echo "Quick Start:"
echo "  curl $RAYNE_URL/health"
echo "  curl -X POST $RAYNE_URL/v1/demo/seed/all"
echo ""
echo "View logs:"
echo "  kubectl logs -f -l app=rayne"
echo ""
echo "Access PostgreSQL:"
echo "  kubectl exec -it \$(kubectl get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U rayne -d rayne"
echo ""

# Verify server is responding
echo "=== Verifying Server Health ==="
echo ""
MAX_RETRIES=10
RETRY_COUNT=0
HEALTH_OK=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    echo "Checking health endpoint (attempt $((RETRY_COUNT + 1))/$MAX_RETRIES)..."
    if curl -s --connect-timeout 5 "$RAYNE_URL/health" > /dev/null 2>&1; then
        HEALTH_OK=true
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    sleep 3
done

if [ "$HEALTH_OK" = true ]; then
    echo ""
    echo "✓ Server is healthy and responding!"
    echo ""
    echo "Health check response:"
    curl -s "$RAYNE_URL/health" | head -c 500
    echo ""
else
    echo ""
    echo "⚠ Warning: Server health check failed after $MAX_RETRIES attempts"
    echo "Check the logs: kubectl logs -f -l app=rayne"
fi

echo ""
echo "=== Checking Datadog Agent Status ==="
if kubectl get pods -l app=datadog-agent 2>/dev/null | grep -q Running; then
    echo "✓ Datadog Agent pods are running"
    kubectl get pods -l app=datadog-agent
else
    echo "⚠ Datadog Agent pods may not be running yet"
    kubectl get pods -l app=datadog-agent 2>/dev/null || echo "  No Datadog Agent pods found"
fi

echo ""
echo "=== Verifying APM Trace Connectivity ==="
# Generate a request to trigger tracer initialization
curl -s "$RAYNE_URL/health" > /dev/null 2>&1
sleep 3

# Check for trace errors in Rayne logs
if kubectl logs -l app=rayne --tail=50 2>&1 | grep -q "Datadog APM tracer started"; then
    echo "✓ APM tracer started successfully"
    kubectl logs -l app=rayne --tail=50 2>&1 | grep "Datadog APM tracer started"
elif kubectl logs -l app=rayne --tail=50 2>&1 | grep -q "lost.*traces"; then
    echo "⚠ APM trace errors detected - agent may not be reachable"
    kubectl logs -l app=rayne --tail=50 2>&1 | grep "lost.*traces" | tail -1
    echo ""
    echo "Try restarting Rayne: kubectl rollout restart deployment/rayne"
else
    echo "⚠ APM tracer status unknown - check logs"
fi

echo ""
echo "=== Verifying Claude Agent Sidecar ==="
if kubectl logs -l app=rayne -c claude-agent --tail=10 2>&1 | grep -q "Server listening"; then
    echo "✓ Claude Agent sidecar is running"
else
    echo "⚠ Claude Agent sidecar may not be ready yet"
    echo "  Check logs: kubectl logs -l app=rayne -c claude-agent"
fi

echo ""
echo "=== Verifying Qdrant Vector DB ==="
if kubectl get pods -l app=qdrant 2>/dev/null | grep -q Running; then
    echo "✓ Qdrant is running"
else
    echo "⚠ Qdrant may not be ready yet"
fi

echo ""
echo "=== Verifying Ollama (Embeddings) ==="
if kubectl get pods -l app=ollama 2>/dev/null | grep -q Running; then
    echo "✓ Ollama is running"
    echo "  Note: Gemma model may still be downloading..."
else
    echo "⚠ Ollama may not be ready yet"
fi

echo ""
echo "=== Cloudflare Tunnel Setup (Persistent Webhook URL) ==="
echo ""
TUNNEL_CREDS="$HOME/.cloudflared/2d837cb9-22e8-44c7-a8a4-2316157ec9c9.json"
if [ -f "$TUNNEL_CREDS" ]; then
    echo "Found Cloudflare tunnel credentials, setting up persistent webhook URL..."
    kubectl create secret generic cloudflare-tunnel-credentials \
        --from-file=credentials.json="$TUNNEL_CREDS" \
        --dry-run=client -o yaml | kubectl apply -f -
    kubectl apply -f cloudflare-tunnel.yaml
    echo ""
    echo "✓ Cloudflare Tunnel deployed!"
    echo "  Webhook URL: https://webhooks.n0kos.com/v1/webhooks/receive"
    echo ""
    echo "Waiting for tunnel to be ready..."
    kubectl wait --for=condition=available deployment/cloudflare-tunnel --timeout=60s 2>/dev/null || \
        echo "  Note: Tunnel may still be starting..."
else
    echo "Cloudflare tunnel credentials not found at: $TUNNEL_CREDS"
    echo ""
    echo "To set up persistent webhook URL (https://webhooks.n0kos.com):"
    echo "  1. cloudflared tunnel login"
    echo "  2. Credentials should exist at: $TUNNEL_CREDS"
    echo "  3. Re-run this script"
    echo ""
    echo "Alternative options for temporary testing:"
    echo "  - minikube service rayne-service --url"
    echo "  - kubectl port-forward svc/rayne-service 8080:8080"
fi
echo ""

echo ""
echo "=== Setup Complete ==="
