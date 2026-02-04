#!/usr/bin/env bash

# Rayne Minikube Setup Script
# This script sets up the Rayne application in a local minikube cluster
# All configuration is done through interactive gum prompts

set -e

# Resolve absolute path to script directory (works even after cd commands)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

#=============================================================================
# COLORS & STYLING
#=============================================================================
capColor() { gum style --foreground "#118DFF" "$1"; }
redColor() { gum style --foreground "#D82C20" "$1"; }
greenColor() { gum style --foreground "#00FF00" "$1"; }
purpleColor() { gum style --foreground "#9400D3" "$1"; }

#=============================================================================
# HOST DATADOG AGENT APM CONFIGURATION (for docker-compose users)
#=============================================================================
# This function ensures the host Datadog agent is configured to accept
# non-local APM traffic when running with docker-compose. This is required
# because Docker containers access the host agent via host.docker.internal
# which is considered non-local traffic.
#
# Requirements for docker-compose with host Datadog agent:
# 1. /etc/datadog-agent/datadog.yaml must have:
#    apm_config:
#      apm_non_local_traffic: true
# 2. The Datadog agent must be running: sudo systemctl status datadog-agent
# 3. Port 8126 must be listening: ss -tlnp | grep 8126
#
# Note: If minikube isn't running, comment out kubernetes-related config lines
# like extra_listeners, kubernetes_kubelet_host to prevent agent startup errors.
ensure_host_agent_apm() {
    if [ -f /etc/datadog-agent/datadog.yaml ]; then
        if ! grep -q "apm_non_local_traffic: true" /etc/datadog-agent/datadog.yaml; then
            echo "Configuring Datadog agent for non-local APM traffic..."
            echo -e "\napm_config:\n  apm_non_local_traffic: true" | sudo tee -a /etc/datadog-agent/datadog.yaml
            sudo systemctl restart datadog-agent
            echo "Host Datadog agent configured and restarted"
        else
            echo "Host Datadog agent already configured for non-local APM traffic"
        fi
    else
        echo "Note: Host Datadog agent not found at /etc/datadog-agent/datadog.yaml"
        echo "      APM will use in-cluster Datadog agent when minikube is running"
    fi
}

#=============================================================================
# CHECK DEPENDENCIES
#=============================================================================
if ! command -v gum &> /dev/null; then
    echo "Error: gum is not installed"
    echo "Install gum: https://github.com/charmbracelet/gum"
    exit 1
fi

if ! command -v minikube &> /dev/null; then
    echo "Error: minikube is not installed. Please install minikube first."
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed. Please install kubectl first."
    exit 1
fi

#=============================================================================
# DEFAULTS
#=============================================================================
SUBAGENT_API_KEY=""
SUBAGENT_APP_KEY=""
SUBAGENT_SITE="datadoghq.com"
RAYNE_SITE="datadoghq.com"
RAYNE_API_KEY=""
RAYNE_APP_KEY=""
RAYNE_KEY_MODE="default"
CLAUDE_AUTH_MODE=""
ANTHROPIC_API_KEY_VALUE=""
CLAUDE_CREDS_FILE=""

#=============================================================================
# INTERACTIVE SETUP (GUM PROMPTS)
#=============================================================================
gum style \
    --border double \
    --padding "1" \
    --border-foreground "#9400D3" \
    "Rayne Minikube Setup"

#-----------------------------------------------------------------------------
# SUB-AGENT CONFIGURATION
# The Sub-Agent creates incident report notebooks and queries logs for RCA.
# Required APP KEY scopes: notebooks_write, logs_read_data, monitors_read
#-----------------------------------------------------------------------------
echo ""
gum style --faint "Sub-Agent: Creates incident notebooks & queries logs for RCA"
gum style --faint "Required APP_KEY scopes: notebooks_write, logs_read_data, monitors_read"
echo ""
echo "$(purpleColor "Step 1:") Choose Datadog $(redColor "domain") for $(capColor "Sub-Agent")"
SITE_CHOICE=$(gum choose "Commercial (app.datadoghq.com)" "Government (app.ddog-gov.com)")
if [[ "$SITE_CHOICE" == *"Government"* ]]; then
    SUBAGENT_SITE="ddog-gov.com"
else
    SUBAGENT_SITE="datadoghq.com"
fi
echo "  Selected: $(greenColor "$SUBAGENT_SITE")"

echo ""
echo "$(purpleColor "Step 2:") Choose $(redColor "API Key") source for $(capColor "Sub-Agent")"
gum style --faint "  API Key: Used for authentication (any valid key for this org)"
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
gum style --faint "  APP Key MUST have scopes: notebooks_write, logs_read_data, monitors_read"
gum style --faint "  Check scopes at: https://app.${SUBAGENT_SITE}/organization-settings/application-keys"
APP_KEY_LIST=$(printf "Enter Manually\n"; env | grep -i "DD_APP_KEY\|DD_APP\|APP_KEY" | grep -v "^_" | cut -d= -f1 | sort -u)
APP_KEY_CHOICE=$(echo "$APP_KEY_LIST" | gum filter --placeholder "Search APP key env vars...")
if [ "$APP_KEY_CHOICE" = "Enter Manually" ]; then
    SUBAGENT_APP_KEY=$(gum input --placeholder "Enter DD_APP_KEY" --password)
else
    SUBAGENT_APP_KEY="${!APP_KEY_CHOICE}"
    echo "  Using: $(greenColor "$APP_KEY_CHOICE")"
fi

#-----------------------------------------------------------------------------
# RAYNE SERVICE CONFIGURATION
# Rayne sends APM traces, DBM queries, and logs to Datadog via the Agent.
# Required APP KEY scopes: apm_read (optional), logs_read_data (optional)
# The Datadog Agent uses these keys - standard permissions are usually sufficient.
#-----------------------------------------------------------------------------
echo ""
gum style --faint "Rayne Service: Sends APM traces, DBM queries, and logs to Datadog"
gum style --faint "Standard API/APP key permissions are sufficient (used by Datadog Agent)"
echo ""
echo "$(purpleColor "Step 4:") Choose $(redColor "Rayne Service") key configuration"
RAYNE_CHOICE=$(gum choose "Default (use TF_VAR_ecco_dd_* keys)" "Same as Sub-Agent (use keys from above)" "Select Different Keys")
if [[ "$RAYNE_CHOICE" == *"Same"* ]]; then
    RAYNE_KEY_MODE="same"
elif [[ "$RAYNE_CHOICE" == *"Different"* ]]; then
    RAYNE_KEY_MODE="custom"

    echo ""
    echo "$(purpleColor "Step 4a:") Choose $(redColor "API Key") for $(capColor "Rayne Service")"
    gum style --faint "  API Key: Used by Datadog Agent for data ingestion"
    RAYNE_API_KEY_LIST=$(printf "Enter Manually\n"; env | grep -i "DD_API_KEY\|DD_API\|API_KEY" | grep -v "^_" | cut -d= -f1 | sort -u)
    RAYNE_API_KEY_CHOICE=$(echo "$RAYNE_API_KEY_LIST" | gum filter --placeholder "Search API key env vars...")
    if [ "$RAYNE_API_KEY_CHOICE" = "Enter Manually" ]; then
        RAYNE_API_KEY=$(gum input --placeholder "Enter Rayne DD_API_KEY" --password)
    else
        RAYNE_API_KEY="${!RAYNE_API_KEY_CHOICE}"
        echo "  Using: $(greenColor "$RAYNE_API_KEY_CHOICE")"
    fi

    echo ""
    echo "$(purpleColor "Step 4b:") Choose $(redColor "APP Key") for $(capColor "Rayne Service")"
    gum style --faint "  APP Key: Used by Datadog Agent (standard permissions OK)"
    RAYNE_APP_KEY_LIST=$(printf "Enter Manually\n"; env | grep -i "DD_APP_KEY\|DD_APP\|APP_KEY" | grep -v "^_" | cut -d= -f1 | sort -u)
    RAYNE_APP_KEY_CHOICE=$(echo "$RAYNE_APP_KEY_LIST" | gum filter --placeholder "Search APP key env vars...")
    if [ "$RAYNE_APP_KEY_CHOICE" = "Enter Manually" ]; then
        RAYNE_APP_KEY=$(gum input --placeholder "Enter Rayne DD_APP_KEY" --password)
    else
        RAYNE_APP_KEY="${!RAYNE_APP_KEY_CHOICE}"
        echo "  Using: $(greenColor "$RAYNE_APP_KEY_CHOICE")"
    fi
else
    RAYNE_KEY_MODE="default"
fi
echo "  Selected: $(greenColor "$RAYNE_KEY_MODE")"

echo ""
echo "$(purpleColor "Step 5:") Choose $(redColor "Rayne Service") site for APM/DBM/logs"
RAYNE_SITE_CHOICE=$(gum choose "Same as Sub-Agent ($SUBAGENT_SITE)" "Commercial (datadoghq.com)" "Government (ddog-gov.com)")
if [[ "$RAYNE_SITE_CHOICE" == *"Same"* ]]; then
    RAYNE_SITE="$SUBAGENT_SITE"
elif [[ "$RAYNE_SITE_CHOICE" == *"Government"* ]]; then
    RAYNE_SITE="ddog-gov.com"
else
    RAYNE_SITE="datadoghq.com"
fi
echo "  Selected: $(greenColor "$RAYNE_SITE")"

echo ""
echo "$(purpleColor "Step 6:") Configure $(redColor "Claude Authentication")"
CLAUDE_AUTH_CHOICE=$(gum choose \
    "Use existing token (~/.claude/.credentials.json)" \
    "Generate new token (run 'claude login')" \
    "Use API key (ANTHROPIC_API_KEY env var)")

case "$CLAUDE_AUTH_CHOICE" in
    *"existing token"*)
        CLAUDE_CREDS_FILE="$HOME/.claude/.credentials.json"
        if [ ! -f "$CLAUDE_CREDS_FILE" ]; then
            echo "$(redColor "Error:") Credentials file not found at $CLAUDE_CREDS_FILE"
            echo "Run 'claude login' first or choose another option"
            exit 1
        fi
        CLAUDE_AUTH_MODE="token"
        echo "  Using: $(greenColor "OAuth token from $CLAUDE_CREDS_FILE")"
        ;;
    *"Generate new"*)
        echo "Opening Claude login..."
        claude login
        CLAUDE_CREDS_FILE="$HOME/.claude/.credentials.json"
        if [ ! -f "$CLAUDE_CREDS_FILE" ]; then
            echo "$(redColor "Error:") Login failed - credentials file not created"
            exit 1
        fi
        CLAUDE_AUTH_MODE="token"
        echo "  Generated: $(greenColor "New OAuth token")"
        ;;
    *"API key"*)
        ANTHROPIC_KEY_LIST=$(printf "Enter Manually\n"; env | grep -i "ANTHROPIC" | cut -d= -f1 | sort -u)
        ANTHROPIC_KEY_CHOICE=$(echo "$ANTHROPIC_KEY_LIST" | gum filter --placeholder "Search ANTHROPIC env vars...")
        if [ "$ANTHROPIC_KEY_CHOICE" = "Enter Manually" ]; then
            ANTHROPIC_API_KEY_VALUE=$(gum input --placeholder "Enter ANTHROPIC_API_KEY" --password)
        else
            ANTHROPIC_API_KEY_VALUE="${!ANTHROPIC_KEY_CHOICE}"
            echo "  Using: $(greenColor "$ANTHROPIC_KEY_CHOICE")"
        fi
        CLAUDE_AUTH_MODE="apikey"
        ;;
esac

#=============================================================================
# SET RAYNE KEYS BASED ON MODE
#=============================================================================
if [ "$RAYNE_KEY_MODE" = "same" ]; then
    RAYNE_API_KEY="$SUBAGENT_API_KEY"
    RAYNE_APP_KEY="$SUBAGENT_APP_KEY"
elif [ "$RAYNE_KEY_MODE" = "custom" ]; then
    if [ -z "$RAYNE_API_KEY" ] || [ -z "$RAYNE_APP_KEY" ]; then
        echo "Error: Rayne keys not set in custom mode"
        exit 1
    fi
else
    # Default mode - use TF_VAR_ecco_dd_* keys
    if [ -z "$TF_VAR_ecco_dd_api_key" ]; then
        echo "Error: TF_VAR_ecco_dd_api_key not set (required for 'default' Rayne key mode)"
        echo "Set it: export TF_VAR_ecco_dd_api_key=your-api-key"
        exit 1
    fi
    if [ -z "$TF_VAR_ecco_dd_app_key" ]; then
        echo "Error: TF_VAR_ecco_dd_app_key not set (required for 'default' Rayne key mode)"
        echo "Set it: export TF_VAR_ecco_dd_app_key=your-app-key"
        exit 1
    fi
    RAYNE_API_KEY="$TF_VAR_ecco_dd_api_key"
    RAYNE_APP_KEY="$TF_VAR_ecco_dd_app_key"
fi

#=============================================================================
# DISPLAY CONFIGURATION
#=============================================================================
echo ""
echo "=== Rayne Minikube Setup ==="
echo ""
echo "Configuration:"
echo "  Sub-Agent Site:     $SUBAGENT_SITE"
echo "  Sub-Agent API Key:  ${SUBAGENT_API_KEY:0:8}..."
echo "  Sub-Agent APP Key:  ${SUBAGENT_APP_KEY:0:8}..."
echo "  Rayne Site:         $RAYNE_SITE"
echo "  Rayne Key Mode:     $RAYNE_KEY_MODE"
echo "  Rayne API Key:      ${RAYNE_API_KEY:0:8}..."
echo "  Rayne APP Key:      ${RAYNE_APP_KEY:0:8}..."
echo "  Claude Auth Mode:   $CLAUDE_AUTH_MODE"
if [ "$CLAUDE_AUTH_MODE" = "token" ]; then
    echo "  Claude Creds:       $CLAUDE_CREDS_FILE"
elif [ "$CLAUDE_AUTH_MODE" = "apikey" ]; then
    echo "  Anthropic Key:      ${ANTHROPIC_API_KEY_VALUE:0:8}..."
fi
echo ""
gum style --foreground "#FFA500" --bold "Key Permission Reminder:"
gum style --faint "  Sub-Agent APP Key needs: notebooks_write, logs_read_data, monitors_read"
gum style --faint "  If you get 403 errors creating notebooks, check APP Key scopes at:"
gum style --faint "  https://app.${SUBAGENT_SITE}/organization-settings/application-keys"
echo ""

#=============================================================================
# START MINIKUBE
#=============================================================================
if ! minikube status &> /dev/null; then
    echo "Starting minikube..."
    minikube start --driver=docker --cpus=4 --memory=12288
else
    echo "Minikube is already running"
    echo "Note: For full functionality, ensure minikube has at least 12GB RAM"
    echo "Restart with: minikube delete && minikube start --cpus=4 --memory=12288"
fi

#=============================================================================
# BUILD DOCKER IMAGES
#=============================================================================
echo ""
echo "Building Rayne Docker image..."
cd "$PROJECT_DIR/mkii_ddog_server"
DOCKER_BUILDKIT=1 docker build -t rayne:latest .

echo ""
echo "Building Claude Agent Docker image..."
cd "$PROJECT_DIR"
DOCKER_BUILDKIT=1 docker build -t claude-agent:latest -f docker/claude-agent/Dockerfile .

echo ""
echo "Loading images into minikube..."
minikube image load rayne:latest
minikube image load claude-agent:latest

export IMAGE_NAME="rayne:latest"

#=============================================================================
# APPLY KUBERNETES MANIFESTS
#=============================================================================
echo ""
echo "Applying Kubernetes manifests..."
cd "$PROJECT_DIR/k8s"

kubectl apply -f postgres-deployment.yaml
echo "Waiting for PostgreSQL pod to be created..."
sleep 5
until kubectl get pods -l app=postgres 2>/dev/null | grep -q postgres; do
    echo "  Waiting for pod to appear..."
    sleep 2
done
echo "Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=ready pod -l app=postgres --timeout=120s

#=============================================================================
# CREATE SECRETS
#=============================================================================
echo "Creating Sub-Agent Datadog secrets..."
kubectl create secret generic subagent-datadog-secrets \
    --from-literal=api-key="$SUBAGENT_API_KEY" \
    --from-literal=app-key="$SUBAGENT_APP_KEY" \
    --from-literal=site="$SUBAGENT_SITE" \
    --dry-run=client -o yaml | kubectl apply -f -

echo "Creating Rayne Datadog secrets..."
kubectl create secret generic datadog-secrets \
    --from-literal=api-key="$RAYNE_API_KEY" \
    --from-literal=app-key="$RAYNE_APP_KEY" \
    --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "Creating Claude authentication secrets..."
if [ "$CLAUDE_AUTH_MODE" = "token" ]; then
    kubectl create secret generic claude-credentials \
        --from-file=credentials.json="$CLAUDE_CREDS_FILE" \
        --dry-run=client -o yaml | kubectl apply -f -
    echo "✓ Claude OAuth credentials configured from $CLAUDE_CREDS_FILE"
    # Delete anthropic-secrets if exists (so ANTHROPIC_API_KEY won't be set)
    kubectl delete secret anthropic-secrets 2>/dev/null || true
elif [ "$CLAUDE_AUTH_MODE" = "apikey" ]; then
    kubectl create secret generic anthropic-secrets \
        --from-literal=api-key="$ANTHROPIC_API_KEY_VALUE" \
        --dry-run=client -o yaml | kubectl apply -f -
    echo "✓ Anthropic API key configured"
    echo '{}' > /tmp/placeholder-credentials.json
    kubectl create secret generic claude-credentials \
        --from-file=credentials.json=/tmp/placeholder-credentials.json \
        --dry-run=client -o yaml | kubectl apply -f -
    rm /tmp/placeholder-credentials.json
fi

#=============================================================================
# DEPLOY SUPPORTING SERVICES
#=============================================================================
echo ""
echo "Applying assets ConfigMap..."
kubectl apply -f assets-configmap.yaml

echo ""
echo "=== Deploying Qdrant Vector DB ==="
kubectl apply -f qdrant-deployment.yaml
echo "Waiting for Qdrant to be ready..."
kubectl wait --for=condition=ready pod -l app=qdrant --timeout=120s 2>/dev/null || \
    echo "  Note: Qdrant pod may still be starting..."

echo ""
echo "=== Deploying Ollama (Gemma 2B) ==="
kubectl apply -f ollama-deployment.yaml
echo "Note: Ollama will download Gemma model on first start (~1.5GB)"
echo "This may take several minutes..."
kubectl wait --for=condition=ready pod -l app=ollama --timeout=300s 2>/dev/null || \
    echo "  Note: Ollama pod may still be downloading the model..."

#=============================================================================
# INSTALL DATADOG AGENT
#=============================================================================
echo ""
echo "=== Installing Datadog Agent ==="

if ! command -v helm &> /dev/null; then
    echo "Warning: helm is not installed. Skipping Datadog Agent installation."
    echo "To install helm: https://helm.sh/docs/intro/install/"
    echo "APM tracing will not work without the Datadog Agent."
else
    echo "Adding Datadog Helm repository..."
    helm repo add datadog https://helm.datadoghq.com 2>/dev/null || true
    helm repo update

    if helm list | grep -q datadog-agent; then
        echo "Upgrading existing Datadog Agent..."
        helm upgrade datadog-agent datadog/datadog \
            --set datadog.apiKey="$RAYNE_API_KEY" \
            --set datadog.appKey="$RAYNE_APP_KEY" \
            --set datadog.clusterName=rayne \
            --set datadog.site="$RAYNE_SITE" \
            --set agents.enabled=true \
            --set clusterAgent.enabled=true \
            -f "$PROJECT_DIR/helm/values.yaml"
    else
        echo "Installing Datadog Agent..."
        helm install datadog-agent datadog/datadog \
            --set datadog.apiKey="$RAYNE_API_KEY" \
            --set datadog.appKey="$RAYNE_APP_KEY" \
            --set datadog.clusterName=rayne \
            --set datadog.site="$RAYNE_SITE" \
            --set agents.enabled=true \
            --set clusterAgent.enabled=true \
            -f "$PROJECT_DIR/helm/values.yaml"
    fi

    echo "Waiting for Datadog Agent to be ready..."
    kubectl wait --for=condition=ready pod -l app=datadog-agent --timeout=180s 2>/dev/null || \
        echo "  Note: Datadog Agent pods may still be starting..."

    echo "Waiting for Datadog Agent service to be available..."
    until kubectl get svc datadog-agent 2>/dev/null | grep -q datadog-agent; do
        echo "  Waiting for datadog-agent service..."
        sleep 2
    done
    echo "✓ Datadog Agent service is available"

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

#=============================================================================
# DEPLOY RAYNE
#=============================================================================
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

#=============================================================================
# DEPLOYMENT COMPLETE
#=============================================================================
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

#=============================================================================
# VERIFY DEPLOYMENT
#=============================================================================
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
curl -s "$RAYNE_URL/health" > /dev/null 2>&1
sleep 3

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
