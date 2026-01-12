#!/usr/bin/env bash

# Rayne Minikube Setup Script
# This script sets up the Rayne application in a local minikube cluster

set -e

echo "=== Rayne Minikube Setup ==="
echo ""

# Check for required environment variables
if [ -z "$TF_VAR_ecco_dd_api_key" ]; then
    echo "Error: DD_API_KEY environment variable is not set"
    echo "Please set it: export DD_API_KEY=your-api-key"
    exit 1
fi

if [ -z "$TF_VAR_ecco_dd_app_key" ]; then
    echo "Error: DD_APP_KEY environment variable is not set"
    echo "Please set it: export DD_APP_KEY=your-app-key"
    exit 1
fi

echo "Datadog API keys found in environment"

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
    minikube start --driver=docker --cpus=2 --memory=4096
else
    echo "Minikube is already running"
fi

# Build the Docker image locally using buildx
echo ""
echo "Building Rayne Docker image..."
cd "$(dirname "$0")/../mkii_ddog_server"
DOCKER_BUILDKIT=1 docker build -t rayne:latest .

# Load image into minikube (works for both single and multi-node)
echo ""
echo "Loading image into minikube..."
minikube image load rayne:latest

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

# Create datadog secrets from environment variables (not from file with placeholders)
echo "Creating Datadog secrets from environment variables..."
kubectl create secret generic datadog-secrets \
    --from-literal=api-key="$TF_VAR_ecco_dd_api_key" \
    --from-literal=app-key="$TF_VAR_ecco_dd_app_key" \
    --dry-run=client -o yaml | kubectl apply -f -

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
            --set datadog.apiKey="$TF_VAR_ecco_dd_api_key" \
            --set datadog.appKey="$TF_VAR_ecco_dd_app_key" \
            --set datadog.clusterName=rayne \
            --set datadog.site=datadoghq.com \
            --set agents.enabled=true \
            --set clusterAgent.enabled=true \
            -f "$(dirname "$0")/../helm/values.yaml"
    else
        echo "Installing Datadog Agent..."
        helm install datadog-agent datadog/datadog \
            --set datadog.apiKey="$TF_VAR_ecco_dd_api_key" \
            --set datadog.appKey="$TF_VAR_ecco_dd_app_key" \
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
echo "=== Setup Complete ==="
