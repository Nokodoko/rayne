# agentic_instructions.md

## Purpose
Kubernetes manifests for deploying the full Rayne stack on minikube: Rayne server, PostgreSQL, Ollama, Qdrant, Cloudflare tunnel, and associated secrets/configmaps.

## Technology
Kubernetes YAML, Deployments, Services, Secrets, ConfigMaps, PVCs

## Contents
- `rayne-deployment.yaml` -- Rayne Go server Deployment + Service (port 8080)
- `postgres-deployment.yaml` -- PostgreSQL Deployment + Service + PVC
- `ollama-deployment.yaml` -- Ollama LLM server Deployment + Service
- `qdrant-deployment.yaml` -- Qdrant vector DB Deployment + Service + PVC
- `cloudflare-tunnel.yaml` -- Cloudflare tunnel Deployment for external routing (n0kos.com, webhooks.n0kos.com)
- `datadog-secrets.yaml` -- Secret: DD_API_KEY, DD_APP_KEY
- `anthropic-secrets.yaml` -- Secret: ANTHROPIC_API_KEY
- `assets-configmap.yaml` -- ConfigMap: incident report templates mounted into containers
- `ngrok-tunnel.yaml` -- Alternative ngrok tunnel (not actively used)

## Key Functions
N/A (declarative YAML)

## Data Types
N/A

## Logging
N/A

## CRUD Entry Points
- **Create**: Add new YAML manifest, `kubectl apply -f <file>`
- **Read**: `kubectl get pods/services/deployments`
- **Update**: Edit YAML, `kubectl apply -f <file>`, `kubectl rollout restart deployment/<name>`
- **Delete**: `kubectl delete -f <file>`

## Style Guide
- One resource type per file (deployment + service together)
- Secrets base64 encoded, referenced via secretKeyRef in envFrom
- PVCs with hostPath for minikube development
