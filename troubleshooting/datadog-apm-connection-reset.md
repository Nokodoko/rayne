# Datadog APM Connection Reset Errors in Kubernetes

## Symptoms

The Rayne application logs showed repeated connection errors when sending traces to the Datadog Agent:

```
Datadog Tracer v1.74.8 ERROR: Error sending stats payload: Post "http://192.168.49.2:8126/v0.6/stats":
read tcp 10.244.0.21:55096->192.168.49.2:8126: read: connection reset by peer
```

The tracer was attempting to connect to `192.168.49.2:8126` (the minikube node IP) but connections were being reset.

## Investigation

### Step 1: Verify Datadog Agent Status

First, checked if the Datadog Agent pods were running:

```bash
kubectl get pods --all-namespaces | grep datadog
```

Result: Agent pods were running (`datadog-agent-gzwg6` with 3/3 containers).

### Step 2: Check Agent Network Configuration

Checked if the Datadog Agent DaemonSet was using `hostNetwork`:

```bash
kubectl get daemonset datadog-agent -o jsonpath='{.spec.template.spec.hostNetwork}'
```

Result: Empty (false) - the agent was **not** using hostNetwork.

### Step 3: Check Available Services

Listed Datadog-related services:

```bash
kubectl get svc | grep datadog
```

Result:
```
datadog-agent                   ClusterIP   10.104.219.233   <none>   8125/UDP,8126/TCP
datadog-agent-cluster-agent     ClusterIP   10.97.11.149     <none>   5005/TCP
```

The agent was exposed via a ClusterIP service at `datadog-agent:8126`.

### Step 4: Test Connectivity from Rayne Pod

Verified the agent was reachable using the service name:

```bash
kubectl exec -it <rayne-pod> -- wget -qO- http://datadog-agent:8126/info
```

Result: Success - the agent responded with its configuration.

### Step 5: Check Rayne Deployment Configuration

Examined the `DD_AGENT_HOST` environment variable:

```bash
kubectl get deployment rayne -o jsonpath='{.spec.template.spec.containers[0].env}' | jq '.'
```

Result:
```json
{
  "name": "DD_AGENT_HOST",
  "valueFrom": {
    "fieldRef": {
      "fieldPath": "status.hostIP"
    }
  }
}
```

The deployment was configured to use `status.hostIP`, which resolved to `192.168.49.2` (the node IP).

## Root Cause

The Rayne deployment was configured to reach the Datadog Agent using the Kubernetes node's host IP (`status.hostIP`). This configuration assumes the Datadog Agent DaemonSet is running with `hostNetwork: true`, which exposes port 8126 directly on the node.

However, the Helm-deployed Datadog Agent was **not** using hostNetwork. Instead, it was only accessible via the ClusterIP service `datadog-agent`.

```
Expected:  Pod -> 192.168.49.2:8126 -> Agent (via hostNetwork)
Actual:    Pod -> 192.168.49.2:8126 -> Nothing listening -> Connection reset
```

## Solution

Changed the `DD_AGENT_HOST` environment variable from using `status.hostIP` to the static service name:

**Before (rayne-deployment.yaml):**
```yaml
- name: DD_AGENT_HOST
  valueFrom:
    fieldRef:
      fieldPath: status.hostIP
```

**After:**
```yaml
- name: DD_AGENT_HOST
  value: "datadog-agent"
```

Applied the fix:
```bash
kubectl apply -f k8s/rayne-deployment.yaml
kubectl rollout restart deployment/rayne
```

## Verification

After the fix, the tracer configuration showed the correct agent URL:

```
agent_url: http://datadog-agent:8126/v0.4/traces
agent_error: ""
Datadog APM tracer started: service=rayne env=staging version=1.0.0 agent=datadog-agent
```

No more connection reset errors in the logs.

## When to Use Each Approach

| Agent Configuration | DD_AGENT_HOST Value |
|---------------------|---------------------|
| DaemonSet with `hostNetwork: true` | `status.hostIP` (node IP) |
| DaemonSet with ClusterIP service | `datadog-agent` (service name) |
| Sidecar container | `localhost` |

## Prevention

When deploying the Datadog Agent via Helm, check the `hostNetwork` setting in values.yaml:

```yaml
# helm/values.yaml
agents:
  useHostNetwork: true  # Set to true if using status.hostIP
```

If `useHostNetwork` is false (default in many configurations), applications must use the service name instead of the host IP.

---

# Additional Issue: TLS Certificate Validation Failures (System Clock)

## Symptoms

After fixing the connection reset issue, traces are received by the agent but not appearing in Datadog. The agent status shows:

```
Receiver (previous minute)
  Traces received: 9 (5,679 bytes)

Writer (previous minute)
  Traces: 0 payloads, 0 traces, 0 events, 0 bytes
```

## Investigation

Checked trace-agent logs for errors:

```bash
kubectl logs <agent-pod> -c trace-agent | grep -iE "error|warn|drop|tls"
```

Found TLS certificate validation failures:

```
WARN | Dropping Payload after 4 retries, due to: Post "https://trace.agent.datadoghq.com/api/v0.2/traces":
tls: failed to verify certificate: x509: certificate has expired or is not yet valid:
current time 2026-01-08T22:21:16Z is after 2026-01-07T23:59:59Z
```

## Root Cause

The system clock is set to a future date (2026) for demo/testing purposes, but Datadog's TLS certificates are not valid that far in the future. The certificate chain fails validation because:

1. TLS certificates have a "Not After" validity date
2. The system reports current time as 2026-01-08
3. The certificate's validity ended 2026-01-07T23:59:59Z
4. The agent correctly rejects the "expired" certificate

## Impact

- Traces are received by the local agent (receiver works)
- Traces cannot be forwarded to Datadog (writer fails)
- All trace payloads are dropped after retry attempts
- No errors appear in the application logs (Rayne)
- The agent silently drops data

## Solutions

### Option 1: Fix System Clock (Recommended for Production)

Set the system clock to the actual current date:

```bash
# On the host system
sudo timedatectl set-ntp true

# Or manually
sudo date -s "2025-01-09 14:00:00"
```

### Option 2: Skip TLS Verification (Testing Only)

Add to the Datadog Agent Helm values (NOT recommended for production):

```yaml
# helm/values.yaml - under agents.containers section
agents:
  containers:
    agent:
      env:
        - name: DD_SKIP_SSL_VALIDATION
          value: "true"
    traceAgent:
      env:
        - name: DD_SKIP_SSL_VALIDATION
          value: "true"
```

**Important**: The `DD_SKIP_SSL_VALIDATION` env var must be set in BOTH the `agent` and `traceAgent` containers.

Verify the config is applied:
```bash
# Check env var is set
kubectl exec <agent-pod> -c agent -- env | grep SSL
# Output: DD_SKIP_SSL_VALIDATION=true

# Check config value
kubectl exec <agent-pod> -c agent -- agent config | grep skip_ssl
# Should show: skip_ssl_validation: true
```

### Option 3: Accept Future Dates for Demo Environments

If running with a fake future date intentionally, consider:
- Using a local mock endpoint that doesn't require valid TLS
- Setting up a local APM collector for testing

## Verification

After fixing, verify traces are being sent:

```bash
# Enable debug logging to see actual flush activity
kubectl set env daemonset/datadog-agent DD_LOG_LEVEL=debug -c trace-agent

# Check for successful flushes (more reliable than status)
kubectl logs <agent-pod> -c trace-agent | grep "Flushed"
# Should show:
# Flushed traces to the API; time: 95ms, bytes: 814
# Flushed stats to the API; time: 38ms, bytes: 459

# Note: The "agent status" Writer section may show 0 even when traces
# are being sent successfully. Use debug logs to verify actual delivery.
```

---

# Verifying Trace Delivery with Debug Logs

## Problem

The `agent status` command may show `Writer: 0 payloads, 0 traces` even when traces are being successfully delivered to Datadog. This can be misleading when troubleshooting.

## Solution: Enable Debug Logging

Enable debug logging on the trace-agent to see actual flush activity:

```bash
# Enable debug logging
kubectl set env daemonset/datadog-agent DD_LOG_LEVEL=debug -c trace-agent

# Wait for rollout
kubectl rollout status daemonset/datadog-agent
```

## What to Look For

Check trace-agent logs for flush messages:

```bash
kubectl logs <agent-pod> -c trace-agent | grep -i "Flushed"
```

**Successful output:**
```
Flushed traces to the API; time: 95.368864ms, bytes: 814
Flushed stats to the API; time: 38.708992ms, bytes: 459
Flushed traces to the API; time: 132.101999ms, bytes: 809
```

**What the fields mean:**
- `time`: Round-trip time to Datadog intake endpoint (should be <500ms)
- `bytes`: Payload size sent

## Additional Debug Commands

```bash
# Check trace receiver activity (traces coming IN from your app)
kubectl logs <agent-pod> -c trace-agent | grep "traces received"
# Output: traces received: 9, traces filtered: 0, traces amount: 5679 bytes

# Check for any send errors
kubectl logs <agent-pod> -c trace-agent | grep -iE "(error|warn|drop|fail)" | grep -v cgroupv2

# Full debug output for trace writer
kubectl logs <agent-pod> -c trace-agent | grep -iE "(flush|send|writer|payload)"
```

## Disable Debug Logging

After troubleshooting, disable debug logging to reduce log volume:

```bash
kubectl set env daemonset/datadog-agent DD_LOG_LEVEL=INFO -c trace-agent
```

---

# HTTP 4xx Responses Not Showing as Errors in APM

## Problem

By default, the dd-trace-go HTTP middleware only marks 5xx responses as errors. 4xx responses (400 Bad Request, 404 Not Found, etc.) appear as successful traces in Datadog APM, making it difficult to track client errors.

## Symptoms

- Traffic generator injects 10% failures (404s, 400s)
- Rayne logs show the requests completing
- Datadog APM shows 0% error rate
- All traces appear green/successful

## Root Cause

The `httptrace.WrapHandler()` middleware defaults to only marking HTTP 500+ status codes as errors.

## Solution

Add `httptrace.WithStatusCheck()` option to mark 4xx responses as errors:

**Before (`cmd/api/api.go`):**
```go
tracedRouter := httptrace.WrapHandler(router, "rayne", "/",
    httptrace.WithSpanOptions(),
)
```

**After:**
```go
tracedRouter := httptrace.WrapHandler(router, "rayne", "/",
    httptrace.WithSpanOptions(),
    httptrace.WithStatusCheck(func(statusCode int) bool {
        return statusCode >= 400
    }),
)
```

## Rebuild and Deploy

After making the change:

```bash
# Rebuild the image (with minikube docker env)
eval $(minikube docker-env)
docker build -t rayne:latest ./mkii_ddog_server

# Restart the deployment
kubectl rollout restart deployment/rayne
kubectl rollout status deployment/rayne
```

## Verification

After deployment, 4xx and 5xx responses will appear as errors in Datadog APM:
- Error rate should reflect actual HTTP errors
- Traces with 4xx/5xx will be marked red
- Error tracking and alerting will work for client errors

## Alternative: Mark Only Specific Status Codes

If you only want certain status codes as errors:

```go
httptrace.WithStatusCheck(func(statusCode int) bool {
    // Only 5xx and specific 4xx codes
    return statusCode >= 500 || statusCode == 400 || statusCode == 404
}),
```

---

# Additional Issue: DNS Resolution Failures

## Symptoms

Traces are received but payloads are dropped with DNS errors:

```
WARN | Dropping Payload after 4 retries, due to: Post "https://trace.agent.datadoghq.com/api/v0.2/traces":
dial tcp: lookup trace.agent.datadoghq.com: Temporary failure in name resolution
```

## Root Cause

Network changes (like DHCP client restart) can temporarily break DNS resolution inside Kubernetes pods. The agent cannot resolve external hostnames like `trace.agent.datadoghq.com`.

## Investigation

```bash
# Check CoreDNS is running
kubectl get pods -n kube-system | grep dns

# Check CoreDNS logs for resolution attempts
kubectl logs -n kube-system -l k8s-app=kube-dns --tail=30

# Check resolv.conf in agent pod
kubectl exec <agent-pod> -c agent -- cat /etc/resolv.conf
```

## Solution

1. **Restart DHCP client** on the host:
   ```bash
   sudo systemctl restart dhcpcd
   ```

2. **Restart CoreDNS** if needed:
   ```bash
   kubectl rollout restart deployment/coredns -n kube-system
   ```

3. **Restart the Datadog Agent** to clear cached DNS failures:
   ```bash
   kubectl rollout restart daemonset/datadog-agent
   ```

## Verification

After DNS is restored, the trace-agent logs should show only "traces received" messages without any resolution errors.

---

## Key Takeaway

When traces are received but not forwarded, check:
1. TLS/certificate errors in trace-agent logs
2. System clock synchronization
3. Network connectivity to Datadog endpoints
4. **DNS resolution** (look for "name resolution" errors)
5. API key validity

## Current Configuration (as of troubleshooting)

The following Helm values are configured in `/home/n0ko/Portfolio/rayne/helm/values.yaml`:

```yaml
# datadog.env - global agent env vars
datadog:
  env:
    - name: DD_SKIP_SSL_VALIDATION
      value: "true"

# agents.containers.agent.env - agent container env vars
agents:
  containers:
    agent:
      env:
        - name: DD_SKIP_SSL_VALIDATION
          value: "true"
    traceAgent:
      env:
        - name: DD_SKIP_SSL_VALIDATION
          value: "true"

# agents.customAgentConfig - direct config injection
agents:
  customAgentConfig:
    skip_ssl_validation: true
```

To redeploy with these settings:
```bash
helm upgrade datadog-agent datadog/datadog \
    --set datadog.apiKey="$TF_VAR_ecco_dd_api_key" \
    --set datadog.appKey="$TF_VAR_ecco_dd_app_key" \
    --set datadog.clusterName=rayne \
    --set datadog.site=datadoghq.com \
    -f helm/values.yaml

kubectl rollout restart deployment/rayne
```

---

# Trace Errors After Minikube Reboot

## Symptoms

After restarting minikube or the host machine, Rayne shows trace connection errors:

```
Datadog Tracer v1.74.8 ERROR: lost 9 traces: Post "http://datadog-agent:8126/v0.4/traces":
dial tcp: lookup datadog-agent: no such host
```

Or:

```
Post "http://localhost:8126/v0.4/traces": dial tcp [::1]:8126: connect: connection refused
```

## Root Cause

The `minikube-setup.sh` script was deploying Rayne BEFORE the Datadog Agent. On a fresh minikube start:

1. PostgreSQL deployed first
2. Rayne deployed (tried to connect to non-existent `datadog-agent` service)
3. Datadog Agent deployed
4. Rayne restarted, but sometimes the restart didn't properly reinitialize the tracer

## Solution

Fixed `scripts/minikube-setup.sh` to deploy in the correct order:

1. PostgreSQL
2. Datadog Agent (wait for service to be available)
3. Rayne

The key changes:
- Moved Helm install of Datadog Agent BEFORE Rayne deployment
- Added explicit wait for `datadog-agent` service to be available
- Added APM endpoint readiness check (curl to port 8126) before deploying Rayne
- Removed the "restart Rayne" step (no longer needed since agent is ready first)

## Verification

After running `./scripts/minikube-setup.sh`, the script now:
1. Waits for Datadog Agent pods to be ready
2. Waits for `datadog-agent` service to exist
3. Then deploys Rayne
4. Verifies APM connectivity at the end

## Quick Fix for Existing Deployments

If you see trace errors after a reboot, restart Rayne:

```bash
kubectl rollout restart deployment/rayne
kubectl rollout status deployment/rayne

# Verify traces work
kubectl logs -l app=rayne --tail=20 | grep "Datadog APM tracer started"
```

Expected output:
```
Datadog APM tracer started: service=rayne env=staging version=1.0.0 agent=datadog-agent
```

---

# Agent Service Ready But APM Endpoint Not Responding

## Symptoms

After minikube setup, Rayne shows trace connection errors even though the `datadog-agent` service exists:

```
Datadog Tracer v1.74.8 ERROR: lost 4 traces: Post "http://datadog-agent:8126/v0.4/traces":
dial tcp 10.104.219.233:8126: connect: connection refused
```

The agent pod is running (3/3 containers) but the APM endpoint on port 8126 isn't ready yet.

## Root Cause

The Kubernetes service is created before the agent containers are fully ready to accept connections. The `kubectl wait --for=condition=ready` checks pod readiness, but the trace-agent container may take additional time to start listening on port 8126.

## Solution

Added APM endpoint readiness check in `scripts/minikube-setup.sh`:

```bash
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
```

This ensures the trace-agent is actually accepting connections before deploying Rayne.

## Quick Fix

If you see this after setup, just restart Rayne:

```bash
kubectl rollout restart deployment/rayne
```

## Verification

Test connectivity manually:

```bash
kubectl exec -it $(kubectl get pod -l app=rayne -o jsonpath='{.items[0].metadata.name}') -- \
    wget -q -O- http://datadog-agent:8126/info | head -5
```

Should return:
```json
{
    "version": "7.57.2",
    ...
}
```
