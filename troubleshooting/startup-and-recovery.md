# Rayne Service Startup & Recovery Troubleshooting

**Last Updated**: 2026-02-10

## The Problem: No "One Shot" Startup

There is currently no single command that brings up the entire rayne stack. The components are spread across multiple systems and require manual sequencing. This document captures what actually works when you need to bring everything back up.

## Architecture Overview

```
Internet
  └── Cloudflare Tunnel (k8s pod in minikube)
        ├── n0kos.com / www.n0kos.com  → frontend (Go binary on host:base, port 3000)
        ├── webhooks.n0kos.com         → rayne-service (k8s, port 8080)
        └── gateway.n0kos.com          → monty gateway (host:base, port 8001)

host:base (192.168.50.179)
  ├── minikube (docker driver, --cpus=4 --memory=12288)
  │     ├── rayne pod (2 containers: rayne + claude-agent sidecar)
  │     ├── postgres pod
  │     ├── qdrant pod
  │     ├── ollama pod (gemma 2b)
  │     ├── cloudflare-tunnel pod
  │     ├── datadog-agent (daemonset)
  │     ├── datadog-cluster-agent (2 replicas, HA)
  │     └── datadog-clusterchecks (2 replicas)
  ├── frontend binary (standalone, port 3000, NOT in k8s)
  └── docker-compose rayne (port 8080, independent of k8s stack)
```

**Key insight**: The frontend is a standalone Go binary running directly on host:base — NOT inside minikube. The Cloudflare tunnel pod (inside minikube) reaches it via the minikube host gateway at `192.168.49.1:3000`.

---

## Recovery Procedure: Minikube Was Stopped

This is the most common scenario (e.g., after GHES took over, host reboot, or manual stop).

### Step 1: Start Minikube

```bash
ssh base 'minikube start --driver=docker --cpus=4 --memory=12288'
```

All existing k8s deployments auto-recover from persistent state. No need to re-apply manifests. Wait ~2 minutes for all pods to reach Running status.

```bash
ssh base 'kubectl get pods -A'
```

Expected: all pods in `Running` state. If any are stuck in `Completed` or `Error`, delete them so the deployment recreates them:

```bash
ssh base 'kubectl delete pod <stuck-pod-name>'
```

### Step 2: Verify the Frontend Is Running

The frontend is a standalone process, NOT managed by k8s. Check if it survived:

```bash
ssh base 'ss -tlnp | grep 3000'
```

If nothing is listening on 3000, start it:

```bash
ssh base 'cd ~/Portfolio/rayne/frontend && nohup ./bin/frontend > /tmp/frontend.log 2>&1 &'
```

If the binary is missing or stale, rebuild:

```bash
ssh base 'cd ~/Portfolio/rayne/frontend && go run github.com/a-h/templ/cmd/templ@latest generate && go build -o ./bin/frontend . && nohup ./bin/frontend > /tmp/frontend.log 2>&1 &'
```

### Step 3: Verify n0kos.com Is Reachable

```bash
curl -s -o /dev/null -w "%{http_code}" https://n0kos.com
```

If you get 502/503 or connection refused, check the Cloudflare tunnel logs:

```bash
ssh base 'kubectl logs deployment/cloudflare-tunnel --tail=20'
```

**KNOWN ISSUE**: If the tunnel logs show errors connecting to `192.168.50.68:3000` (monty host), the ConfigMap has the old/wrong origin. See "Cloudflare Tunnel Points to Wrong Origin" below.

### Step 4: Verify All Services

```bash
# Rayne API (k8s)
ssh base 'curl -s http://192.168.49.2:31981/health'

# Frontend
ssh base 'curl -s -o /dev/null -w "%{http_code}" http://localhost:3000'

# Webhooks endpoint
curl -s -o /dev/null -w "%{http_code}" https://webhooks.n0kos.com/health

# Datadog Agent
ssh base 'kubectl exec $(kubectl get pod -l app=datadog-agent -o jsonpath="{.items[0].metadata.name}") -- agent status 2>&1 | head -20'
```

---

## Known Issues & Fixes

### 1. Cloudflare Tunnel Points to Wrong Origin

**Symptom**: n0kos.com returns 502. Tunnel logs show:
```
ERR Unable to reach the origin service ... dial tcp 192.168.50.68:3000: connect: connection refused
```

**Root Cause**: The Cloudflare tunnel ConfigMap (and source YAML at `~/Portfolio/rayne/k8s/cloudflare-tunnel.yaml`) historically pointed n0kos.com to `192.168.50.68:3000` (a host called "monty"). The frontend actually runs on host:base and is reachable from inside minikube at `192.168.49.1:3000`.

**Fix**:
```bash
# Patch the live ConfigMap
ssh base "kubectl get configmap cloudflare-tunnel-config -o json | \
  python3 -c \"import json,sys; cm=json.load(sys.stdin); cm['data']['config.yaml']=cm['data']['config.yaml'].replace('192.168.50.68:3000','192.168.49.1:3000'); json.dump(cm,sys.stdout)\" | \
  kubectl apply -f -"

# Restart the tunnel pod to pick up the change
ssh base 'kubectl rollout restart deployment/cloudflare-tunnel'

# Verify — should see "Registered tunnel connection" with no ERR lines
ssh base 'sleep 10 && kubectl logs deployment/cloudflare-tunnel --tail=10'
```

**Permanent fix**: Also update the source manifest at `~/Portfolio/rayne/k8s/cloudflare-tunnel.yaml` — change `http://192.168.50.68:3000` to `http://192.168.49.1:3000` for both n0kos.com entries.

### 2. minikube-setup.sh Is Interactive Only

**Symptom**: Cannot run `minikube-setup.sh` over SSH or non-interactively.

**Root Cause**: The script uses `gum` (charmbracelet TUI) for interactive prompts to collect Datadog API keys, Claude auth, and configuration choices. There is no `--non-interactive` or `--defaults` flag.

**Workaround**: If minikube was previously set up, simply `minikube start` recovers all deployments. The setup script is only needed for:
- First-time setup
- Changing Datadog API/APP keys
- Changing Claude authentication method
- Rebuilding Docker images

For a cold restart after prior setup:
```bash
minikube start --driver=docker --cpus=4 --memory=12288
# That is it — all deployments, secrets, configmaps persist in minikube state
```

### 3. Datadog DBM PostgreSQL User Missing

**Symptom**: Datadog agent logs show postgres check failures. DBM not collecting query samples.

**Root Cause**: The `datadog` PostgreSQL user for Database Monitoring was never created in the initial setup.

**Fix** (already applied 2026-02-10, should persist across minikube restarts):
```bash
ssh base "kubectl exec -it \$(kubectl get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}') -- psql -U rayne -d rayne" <<'SQL'
CREATE USER datadog WITH PASSWORD 'datadog_dbm_password';
GRANT pg_monitor TO datadog;
GRANT CONNECT ON DATABASE rayne TO datadog;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
GRANT SELECT ON pg_stat_statements TO datadog;
CREATE SCHEMA IF NOT EXISTS datadog;
GRANT USAGE ON SCHEMA datadog TO datadog;
CREATE OR REPLACE FUNCTION datadog.explain_statement(l_query TEXT, OUT explain JSON)
  RETURNS SETOF JSON AS $$
  BEGIN RETURN QUERY EXECUTE pg_catalog.concat('EXPLAIN (FORMAT JSON) ', l_query); END;
$$ LANGUAGE plpgsql RETURNS NULL ON NULL INPUT SECURITY DEFINER;
SQL
```

### 4. Pods Stuck in Completed/Error After Restart

**Symptom**: `kubectl get pods` shows pods in `Completed` or `Error` state instead of `Running`.

**Root Cause**: Some pods (like ollama, cloudflare-tunnel) may not restart cleanly after minikube stop/start.

**Fix**: Delete the stuck pods. The deployment controller recreates them:
```bash
ssh base 'kubectl delete pod <pod-name>'
```

Or bulk-delete all non-running pods:
```bash
ssh base "kubectl get pods --field-selector=status.phase!=Running -o name | xargs -r kubectl delete"
```

### 5. minikube and GHES Cannot Run Simultaneously

**Symptom**: System becomes unresponsive, OOM killer triggers, or services crash.

**Root Cause**: host:base has 32 GB RAM. minikube is capped at 12 GB, GHES needs ~8-12 GB idle. Together they exceed available memory.

**Rule**: Stop one before starting the other:
```bash
# To use GHES:
minikube stop
sudo bash /home/n0ko/ghes/start-ghes.sh

# To use rayne (stop GHES first):
sudo bash /home/n0ko/ghes/stop-ghes.sh
minikube start --driver=docker --cpus=4 --memory=12288
```

---

## Quick Reference: Full Cold Start

After a reboot, GHES shutdown, or any situation where minikube is not running:

```bash
# 1. Ensure GHES is stopped (if it was running)
ssh base 'sudo virsh -c qemu:///system domstate ghes 2>/dev/null | grep -q running && sudo bash /home/n0ko/ghes/stop-ghes.sh || echo "GHES not running"'

# 2. Start minikube (recovers all k8s deployments from persistent state)
ssh base 'minikube start --driver=docker --cpus=4 --memory=12288'

# 3. Wait for pods (~2 min), then clean up stuck ones
ssh base 'sleep 30 && kubectl get pods --field-selector=status.phase!=Running -o name 2>/dev/null | xargs -r kubectl delete'

# 4. Ensure frontend is running (standalone, not in k8s)
ssh base 'ss -tlnp | grep 3000 || (cd ~/Portfolio/rayne/frontend && nohup ./bin/frontend > /tmp/frontend.log 2>&1 &)'

# 5. Verify Cloudflare tunnel (no ERR lines about 192.168.50.68)
ssh base 'sleep 10 && kubectl logs deployment/cloudflare-tunnel --tail=5'

# 6. If tunnel shows wrong origin, fix it
ssh base "kubectl get configmap cloudflare-tunnel-config -o json | python3 -c \"import json,sys; cm=json.load(sys.stdin); c=cm['data']['config.yaml']; cm['data']['config.yaml']=c.replace('192.168.50.68:3000','192.168.49.1:3000') if '192.168.50.68' in c else c; json.dump(cm,sys.stdout)\" | kubectl apply -f - && kubectl rollout restart deployment/cloudflare-tunnel"

# 7. Smoke test
curl -s -o /dev/null -w "n0kos.com: %{http_code}\n" https://n0kos.com
curl -s -o /dev/null -w "webhooks:  %{http_code}\n" https://webhooks.n0kos.com/health
ssh base 'curl -s http://192.168.49.2:31981/health'
```

---

## What Is Still Missing for "One Shot"

The following gaps prevent a single `./start-rayne.sh` from working end-to-end:

1. **Frontend is not managed by k8s or systemd** — it is a standalone Go binary run with `nohup`. If the process dies, nothing restarts it. Needs either a k8s deployment or a systemd user unit.

2. **minikube-setup.sh requires interactive gum prompts** — no non-interactive/headless mode. Cannot be triggered by systemd or cron. A `--non-interactive` flag with env var defaults would solve this.

3. **Cloudflare tunnel source manifest has stale monty IP** — the live ConfigMap was patched but `~/Portfolio/rayne/k8s/cloudflare-tunnel.yaml` still references `192.168.50.68`. Next time the setup script runs and applies manifests, it will overwrite the fix.

4. **No health-check loop after startup** — the recovery steps are manual. Nothing polls services and auto-remediates failed pods or crashed processes.

5. **GHES/minikube mutual exclusion is not enforced** — no guard script prevents starting both simultaneously, risking OOM.

6. **Gateway service (port 8001)** — the monty gateway that serves `gateway.n0kos.com` is not documented in this startup flow. Its startup/management is separate.
