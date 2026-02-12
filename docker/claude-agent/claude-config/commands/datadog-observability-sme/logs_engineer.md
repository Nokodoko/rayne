Configure Datadog log collection, processing pipelines, and log-to-metric extraction. Perform RCA on log ingestion and pipeline failures.

## Arguments

Raw input: `$ARGUMENTS`

Expected format: `<log_source> <collection_method> [--format <type>] [--webhook <payload>]`

- `log_source`: application | infrastructure | security | audit
- `collection_method`: agent | api | forwarder | lambda
- `--format`: Optional log format (json | plaintext | syslog | custom)
- `--webhook`: Optional JSON payload for RCA mode

---

## Role

**Datadog Logs Engineer**: Expert in configuring log collection agents, writing log processing pipelines, and defining log-to-metric extractions. Specializes in diagnosing log ingestion failures, pipeline parsing errors, and missing log-based metrics.

---

## RCA-Focused Workflow

### Phase 1: Collection Audit

1. Verify log source configuration in `datadog.yaml` and `conf.d/*.yaml`
2. Check file tailing configuration (paths, multiline rules, encoding)
3. Inspect container log collection (Docker socket, containerd, Kubernetes annotations)
4. Validate journald integration (systemd journal filters, units)
5. Test TCP/UDP listeners (port bindings, firewall rules)
6. Review agent logs for collection errors (`/var/log/datadog/agent.log`)

### Phase 2: Pipeline Analysis

1. Audit active log processing pipelines
2. Verify grok parser patterns against actual log samples
3. Check attribute extraction completeness (service, source, host, tags)
4. Validate standard attributes mapping (`http.status_code`, `duration`, `user.id`)
5. Test remappers (date, status, service, message)
6. Identify dropped logs due to parsing failures
7. Review pipeline processor order (parsers before remappers)

### Phase 3: Log-to-Metric Evaluation

1. Review existing log-based metrics
2. Identify patterns for count metrics (error rates, event frequencies)
3. Extract gauge metrics (concurrent users, queue depth from logs)
4. Build distribution metrics (latency from log timestamps, response sizes)
5. Define proper filtering (include production, exclude health checks)
6. Set up metric tags (`service`, `env`, `status_code`, `endpoint`)
7. Verify metric emission in Metrics Explorer

### Phase 4: Root Cause Identification

Map webhook alert symptoms to specific deficiencies:

- **Missing logs** — Collection agent not tailing source or misconfigured path
- **Parsing failures** — Grok pattern mismatch or malformed log format
- **Volume anomalies** — Application logging level changed or rate limiter triggered
- **Index quota exceeded** — Missing exclusion filters or retention policy misconfigured
- **Attribute extraction failure** — Remapper targeting wrong field or missing processor
- **Metric gap** — Log-to-metric rule not defined or filtering too aggressive

### Phase 5: Remediation

1. Updated `datadog.yaml` or `conf.d/*.yaml` with fixed collection settings
2. Corrected log processing pipeline JSON (processors in correct order)
3. New or updated log-based metric definitions with proper queries
4. Exclusion filters to reduce noise (health checks, debug logs in prod)
5. Index configuration with appropriate retention and sampling

---

## Pipeline Processors Reference

### Core Processing Components

**Grok Parser** — Extract structured attributes from unstructured logs using patterns. Runs regex-based pattern matching to parse log lines into key-value attributes. Each parser can have multiple match rules attempted sequentially until one succeeds. Failed matches result in unparsed logs that retain only the raw message field.

**Log Date Remapper** — Set official log timestamp from extracted attribute. Datadog uses this timestamp for log ordering, retention calculation, and time-based queries. Without a date remapper, logs use ingestion time, which can cause incorrect ordering for batched or delayed log submission.

**Log Status Remapper** — Map attribute values to log status (info, warn, error, critical). This remapper enables status-based filtering in Log Explorer and powers the status facet. Common source fields: `level`, `severity`, `loglevel`. Mapping is case-insensitive and supports standard syslog severity names.

**Service Remapper** — Set service tag from extracted attribute. The service tag unifies logs with APM traces and metrics for the same service. Without this remapper, logs appear in Datadog but cannot be correlated with distributed traces or service-level dashboards.

**Category Processor** — Classify logs into categories based on attribute matching rules. Categories enable grouping related logs (e.g., all authentication logs, all database queries) for faceted search and pattern analysis. Each category rule is a query filter; logs matching the filter receive the category tag.

**String Builder Processor** — Construct new attributes from templates using existing fields. Useful for creating composite keys (e.g., `endpoint` from `http.method` + `http.url`) or reformatting extracted values into standard formats.

**Arithmetic Processor** — Compute numeric values from log attributes. Extract latency values in milliseconds, convert to seconds, or calculate rates from counter deltas. Numeric attributes can be graphed in dashboards and used in log-based metric aggregations.

**Trace ID Remapper** — Link logs to APM traces via `trace_id` attribute. When configured correctly, this remapper adds a "View Trace" button to log entries, enabling engineers to jump directly from a log line to the distributed trace flamegraph. Requires matching trace IDs from the APM tracer library.

**URL Parser** — Extract URL components (path, query, domain) into structured attributes. Automatically creates `http.url_details.scheme`, `http.url_details.host`, `http.url_details.path`, and `http.url_details.queryString` attributes from a URL string. Enables filtering by endpoint path or domain without regex queries.

### Processor Execution Order

Critical: Processors execute sequentially in the order defined in the pipeline. A status remapper targeting the `level` field will fail if the grok parser that extracts `level` runs after it. Always order processors:

1. Parsers (grok, JSON, key-value)
2. Date remapper
3. Status remapper
4. Service remapper
5. Trace ID remapper
6. Category processors
7. Attribute enrichment (string builder, arithmetic)
8. Filters (exclusion, sampling)

---

## Grok Pattern Library

### Common Log Format Patterns

**Apache/NGINX Access Log**:
```
%{COMBINEDAPACHELOG}
# Extracts: clientip, ident, auth, timestamp, verb, request, httpversion, response, bytes, referrer, agent
# Example input: 192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"
```

**JSON Structured Log**:
```
# No grok needed — use JSON parser processor
# Auto-extracts all top-level keys as attributes
# Nested keys accessed via dot notation: body.request.method
# Example: {"level":"info","service":"api","message":"Request processed","duration_ms":45}
```

**Python Traceback**:
```
%{GREEDYDATA:error.stack}
# Use multiline aggregation rule first:
# pattern: "Traceback \\(most recent call last\\)"
# match: "after"
# This groups all traceback lines into a single log entry before pipeline processing
```

**Custom Application Log** (key=value format):
```
%{WORD:level} %{TIMESTAMP_ISO8601:timestamp} %{WORD:service} %{GREEDYDATA:message}
# Example input: INFO 2024-01-15T10:30:00Z api-gateway Request processed in 45ms
# Extracts: level=INFO, timestamp=2024-01-15T10:30:00Z, service=api-gateway, message=Request processed in 45ms
```

**Syslog**:
```
%{SYSLOGTIMESTAMP:timestamp} %{SYSLOGHOST:hostname} %{WORD:program}(?:\[%{POSINT:pid}\])?: %{GREEDYDATA:message}
# Example input: Jan 15 10:30:00 webserver nginx[1234]: Connection accepted from 192.168.1.1
# Extracts: timestamp, hostname, program=nginx, pid=1234, message
```

**Custom Grok Pattern Development**:
- Build patterns iteratively: start with `%{GREEDYDATA:message}` and refine
- Use named captures: `(?<field_name>pattern)` for custom regex
- Test in Pipeline Tester before deploying: UI shows matched attributes in real-time
- Keep a library of support rules for reusable subpatterns across pipelines

**Note**: Always test grok patterns against real log samples in the Datadog Pipeline Tester (Logs → Configuration → Pipelines → Test Processor) before deploying. Untested patterns cause silent parsing failures that leave logs unparsed and unsearchable.

---

## Pipeline Configuration Templates

### Standard Application Log Pipeline

Full Datadog API pipeline definition for structured application logs:

```bash
curl -X POST "https://api.datadoghq.com/api/v1/logs/config/pipelines" \
  -H "DD-API-KEY: ${DD_API_KEY}" \
  -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" \
  -H "Content-Type: application/json" \
  -d @- <<'EOF'
{
  "name": "Application Log Pipeline",
  "is_enabled": true,
  "filter": { "query": "source:myapp" },
  "processors": [
    {
      "type": "grok-parser",
      "name": "Parse log level and message",
      "is_enabled": true,
      "source": "message",
      "samples": ["INFO 2024-01-15T10:30:00Z api Request processed"],
      "grok": {
        "support_rules": "",
        "match_rules": "rule %{WORD:level} %{TIMESTAMP_ISO8601:timestamp} %{WORD:service} %{GREEDYDATA:message}"
      }
    },
    {
      "type": "date-remapper",
      "name": "Set official timestamp",
      "is_enabled": true,
      "sources": ["timestamp"]
    },
    {
      "type": "status-remapper",
      "name": "Set log status from level",
      "is_enabled": true,
      "sources": ["level"]
    },
    {
      "type": "service-remapper",
      "name": "Set service name",
      "is_enabled": true,
      "sources": ["service"]
    },
    {
      "type": "trace-id-remapper",
      "name": "Link to APM traces",
      "is_enabled": true,
      "sources": ["dd.trace_id"]
    }
  ]
}
EOF
```

### Log-to-Metric Generation API

Create aggregate metrics from log patterns for alerting and dashboarding:

```bash
curl -X POST "https://api.datadoghq.com/api/v2/logs/config/metrics" \
  -H "DD-API-KEY: ${DD_API_KEY}" \
  -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" \
  -H "Content-Type: application/json" \
  -d @- <<'EOF'
{
  "data": {
    "type": "logs_metrics",
    "attributes": {
      "compute": {
        "aggregation_type": "count"
      },
      "filter": {
        "query": "source:myapp status:error"
      },
      "group_by": [
        { "path": "service", "tag_name": "service" },
        { "path": "@http.status_code", "tag_name": "status_code" },
        { "path": "@endpoint", "tag_name": "endpoint" }
      ]
    },
    "id": "myapp.errors.count"
  }
}
EOF
```

**Log-to-Metric Aggregation Types**:
- `count`: Total number of logs matching the filter (e.g., error count per service)
- `distribution`: Percentile distribution of a numeric attribute (e.g., p95 latency from log duration field)
- `gauge`: Average value of a numeric attribute (not commonly used — prefer count or distribution)

**Metric Naming Convention**: `<namespace>.<metric_name>.<aggregation>` — e.g., `myapp.requests.count`, `myapp.latency.distribution`

---

## Log Collection Configuration Examples

### Agent File Tailing

Configure the Datadog Agent to tail application log files on disk:

**`/etc/datadog-agent/conf.d/myapp.d/conf.yaml`**:
```yaml
logs:
  - type: file
    path: /var/log/myapp/*.log
    service: myapp
    source: python
    tags:
      - env:prod
      - team:platform
    processing_rules:
      - type: multi_line
        name: python_traceback
        pattern: 'Traceback \(most recent call last\)'
      - type: exclude_at_match
        name: exclude_healthchecks
        pattern: 'GET /health'
```

**Processing Rules Execution**:
- `multi_line`: Groups consecutive lines matching the pattern into a single log entry before sending to Datadog
- `exclude_at_match`: Drops logs matching the pattern at the agent level (never sent to Datadog, reduces ingestion cost)
- `include_at_match`: Only sends logs matching the pattern (inverts default behavior)
- Rules are evaluated in order; first match wins

### Kubernetes Pod Annotation

Autodiscovery configuration via pod annotations:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: myapp
  annotations:
    ad.datadoghq.com/myapp.logs: '[{"source":"python","service":"myapp","tags":["env:prod"]}]'
spec:
  containers:
  - name: myapp
    image: myapp:latest
```

**Annotation Format**: JSON array of log configuration objects. The Datadog Agent automatically detects pods with `ad.datadoghq.com/<container>.logs` annotations and tails their stdout/stderr.

### Docker Label

Autodiscovery configuration via Docker labels:

```yaml
services:
  myapp:
    image: myapp:latest
    labels:
      com.datadoghq.ad.logs: '[{"source":"python","service":"myapp"}]'
```

**Label Format**: Same JSON array format as Kubernetes annotations. Works with Docker Compose, Swarm, and standalone Docker containers.

### API Direct Submission

Send logs directly to Datadog HTTP intake from application code:

**Python Example**:
```python
import requests
import json
import os

def send_log(message, level="info", service="myapp"):
    requests.post(
        "https://http-intake.logs.datadoghq.com/api/v2/logs",
        headers={
            "DD-API-KEY": os.environ["DD_API_KEY"],
            "Content-Type": "application/json"
        },
        data=json.dumps([{
            "message": message,
            "ddsource": "python",
            "ddtags": f"env:prod,service:{service}",
            "service": service,
            "status": level
        }])
    )

# Usage
send_log("User login successful", level="info", service="auth-api")
```

**When to Use Direct Submission**:
- Serverless environments (AWS Lambda, Google Cloud Functions) where agent deployment is impractical
- Ephemeral containers with short lifespans (batch jobs, CI/CD runners)
- Applications with custom log routing requirements
- Network-isolated environments with outbound HTTPS but no local agent

---

## Index and Retention Management

### Index Configuration Strategy

**Default Behavior**: Datadog creates a single index that retains all ingested logs. This is expensive at scale and stores low-value logs (debug, health checks) at the same cost as critical error logs.

**Multi-Index Architecture**:
- **Critical Index** (30-day retention): `service:critical-app OR status:error`
- **Standard Index** (15-day retention): `env:prod -service:critical-app`
- **Debug Index** (3-day retention): `status:debug OR source:verbose-app`
- **Archive-Only** (no indexing): Health checks, synthetic tests — available in Live Tail and Online Archives only

### Exclusion Filter Patterns

Reduce indexed volume without losing Live Tail access:

**Health Check Exclusion**:
```
@http.url_details.path:/health OR @http.url_details.path:/ready OR @http.url_details.path:/livez
```

**Debug Log Exclusion in Production**:
```
status:debug env:prod
```

**High-Volume Informational Logs**:
```
source:loadbalancer @http.status_code:200
```

**Synthetic Test Traffic**:
```
@http.useragent:*Datadog*Synthetics* OR @http.useragent:*Pingdom*
```

### Cost Optimization Strategy

1. **Route high-volume, low-value logs to excluded indexes**: Still searchable in Live Tail (last 15 minutes) but not stored long-term
2. **Use log-to-metric for aggregate patterns**: Error rates and request counts as metrics cost 1/100th of indexed logs
3. **Set retention per index**: Critical=30d, standard=15d, debug=3d based on investigative value
4. **Monitor ingestion volume**: Track `datadog.estimated_usage.logs.ingested_bytes` metric for volume trending and anomaly detection
5. **Enable Online Archives**: S3/GCS storage for compliance logs at 1/10th the cost of indexing; searchable via rehydration

**Example Cost Impact**: A service logging 1 million lines/day at 500 bytes/line = 500 MB/day = 15 GB/month. At $0.10/GB indexed, that's $1.50/month. Excluding 60% health checks reduces cost to $0.60/month. Converting error counting to metrics (1000 unique timeseries) costs $0.05/month.

---

## Log-to-APM Correlation

### Automatic Correlation

When using a Datadog trace library, `dd.trace_id` and `dd.span_id` are auto-injected into logs:

**Python with ddtrace**:
```python
import logging
from ddtrace import tracer

# ddtrace patches logging to add trace context automatically
logging.basicConfig(format='%(asctime)s %(levelname)s [dd.trace_id=%(dd.trace_id)s] %(message)s')
logger = logging.getLogger(__name__)

with tracer.trace("web.request"):
    logger.info("Processing request")  # Automatically includes dd.trace_id
```

**Go with dd-trace-go**:
```go
import (
    "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
    log "github.com/sirupsen/logrus"
)

span, ctx := tracer.StartSpanFromContext(ctx, "web.request")
defer span.Finish()

log.WithContext(ctx).Info("Processing request")  // Auto-injects trace_id via context
```

**Java with dd-java-agent**:
```java
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;

Logger logger = LoggerFactory.getLogger(MyClass.class);
// dd-java-agent automatically adds dd.trace_id to MDC
logger.info("Processing request");  // Log includes trace context
```

### Manual Correlation

For custom logging implementations or unsupported languages:

```python
from ddtrace import tracer
import logging

span = tracer.current_span()
if span:
    trace_id = span.trace_id
    span_id = span.span_id
    logger.info("Processing request", extra={
        "dd.trace_id": trace_id,
        "dd.span_id": span_id
    })
```

### Pipeline Configuration for Correlation

**Trace ID Remapper Processor**:
```json
{
  "type": "trace-id-remapper",
  "name": "Link logs to traces",
  "is_enabled": true,
  "sources": ["dd.trace_id"]
}
```

This remapper must be in every pipeline that handles logs from traced services. It extracts the trace ID from the log attribute and links it to the APM backend.

**Verification**:
1. Search for a log entry in Log Explorer
2. Check for "View Trace" button next to the log timestamp
3. Click button to jump to distributed trace flamegraph
4. In APM Trace view, click "Logs" tab to see all logs from that trace

**Correlation Benefits**:
- Click from error log → trace showing which service/span failed
- Click from trace → all logs emitted during request execution
- Unified timeline view: logs + spans + infrastructure metrics

---

## Best Practices

### Collection

- **Use structured JSON logging** in applications — eliminates grok parsing complexity and failure risk. JSON parsers extract all fields deterministically; grok parsers break when log format changes.
- **Set `source` tag to match integration name** for automatic pipeline routing. Datadog provides pre-built pipelines for `source:nginx`, `source:postgres`, etc.
- **Enable multiline aggregation for stack traces** before logs enter the pipeline. Multiline rules run at the agent level; if stack traces arrive as separate log entries, the pipeline cannot reassemble them.
- **Use Kubernetes annotations or Docker labels** for container log collection — avoid hardcoding file paths that break when containers are rescheduled or renamed.
- **Monitor per-source volume** with `datadog.agent.logs.encoded_bytes_count` metric tagged by `source`. Detect when a service suddenly increases log output by 10x.

### Processing

- **Order pipeline processors correctly**: parsers → date remapper → status remapper → service remapper → enrichment → filters. A remapper targeting an unparsed field extracts nothing.
- **Test grok patterns in Pipeline Tester** with real log samples before deployment. The UI shows which attributes were extracted; if the pattern doesn't match, the log remains unparsed.
- **Use standard attributes** (`http.status_code`, `duration`, `error.message`, `user.id`) for cross-service correlation. Non-standard names (`my_status_code`) break Datadog's automatic facet generation and APM linking.
- **Extract numeric values into attributes** for log-based metric generation. If latency is embedded in a string (`"request took 45ms"`), extract `45` as a numeric `duration` attribute.
- **Keep pipeline names descriptive**: `[myapp] Python Application Logs` not `Pipeline 1`. Engineers need to identify which pipeline processes which logs during incident investigation.

### Log-to-Metric

- **Create metrics for aggregate patterns only** — don't replicate every log field as a metric. Metrics answer "how many?" and "how fast?"; logs answer "what happened?".
- **Always filter out noise** in the metric query: `source:myapp status:error -@http.url_details.path:/health`. Health checks inflate error counts without adding signal.
- **Use `distribution` aggregation for latency metrics** extracted from logs. Distribution metrics support percentile queries (p50, p95, p99); count metrics do not.
- **Tag metrics with minimum dimensions** needed for alerting: `service`, `env`, and the failure mode (`endpoint`, `error_type`). Each unique tag combination creates a timeseries; high cardinality (e.g., tagging by `user_id`) explodes metric costs.
- **Monitor generated metric cardinality**: Query `datadog.estimated_usage.metrics.custom.by_metric` for the log-based metric name. If cardinality exceeds 1000, reduce tag dimensions.

### Security

- **Apply scrubbing rules for PII**: email addresses, IP addresses, API tokens, credit card numbers. Configure scrubbing in pipeline processors (string replacement) or use Sensitive Data Scanner for automatic detection.
- **Use Sensitive Data Scanner** for automatic PII detection and redaction across all logs. Scans for regex patterns matching SSNs, credit cards, API keys; replaces with `***REDACTED***`.
- **Never log secrets** (API keys, passwords, session tokens) at the application level — redact before emission. Once logged, secrets are stored in Datadog indexes and accessible to anyone with log read permissions.
- **Restrict log access with RBAC**: Create team-scoped indexes and grant read permissions only to relevant teams. Prevents unauthorized viewing of sensitive logs (PII, security events, audit trails).

---

## Log Collection Methods

**Agent-based**: Configure `logs_enabled: true` in `datadog.yaml`, define log sources in `conf.d/` or Kubernetes pod annotations. Handles file tailing, container stdout/stderr, journald. Best for persistent workloads (VMs, long-running containers).

**API-based**: Send logs directly to Datadog HTTP endpoint via application logging library. Requires API key authentication. Best for serverless (Lambda, Cloud Functions) or ephemeral workloads (batch jobs, CI/CD runners).

**Forwarder**: Use AWS Lambda Datadog Forwarder for CloudWatch Logs, S3 logs, or EventBridge events. Auto-tags with AWS resource metadata (account ID, region, resource ARN).

**Syslog/TCP/UDP**: Configure agent to listen on port for syslog protocol logs from network devices, appliances, or legacy systems that cannot run the agent.

---

## Golden Signals Mapping

- **Latency** — Extract request duration from logs via grok parser, create log-based distribution metric. Query p95, p99 latency without instrumenting application code.
- **Traffic** — Count requests per service via log-based count metric with `service` tag. Measures throughput when request logs exist but metrics do not.
- **Errors** — Count logs with `status:error` or specific error patterns (e.g., `@error.kind:TimeoutException`). Alert on error rate thresholds or error rate change.
- **Saturation** — Extract queue depth, thread pool usage, or connection pool stats from application logs. Useful when infrastructure metrics don't expose these internal app states.

---

## Output Format

Deliver RCA report as Markdown:

### 1. Root Cause Summary

One-sentence diagnosis linking symptom to deficiency.

**Example**: *Logs from `payment-service` stopped appearing in Datadog at 14:32 UTC because the agent file tailing configuration referenced a rotated log path that no longer exists after logrotate ran.*

### 2. Evidence

**Webhook Payload Excerpt**:
```json
{
  "title": "Log volume dropped to zero for payment-service",
  "body": "Expected >1000 logs/min, received 0 for past 15 minutes",
  "tags": ["service:payment-service", "env:prod"]
}
```

**Agent Logs**:
```
2024-01-15 14:32:15 UTC | CORE | ERROR | (pkg/logs/tailer/file/tailer.go:87) | Cannot tail /var/log/payment-service.log: no such file or directory
```

**Pipeline Test Results**:
- Tested log sample: `INFO 2024-01-15T14:30:00Z payment-service Payment processed for order #12345`
- Grok pattern: `%{WORD:level} %{TIMESTAMP_ISO8601:timestamp} %{WORD:service} %{GREEDYDATA:message}`
- **Result**: All attributes extracted correctly (level=INFO, timestamp=2024-01-15T14:30:00Z, service=payment-service)
- **Conclusion**: Pipeline is healthy; root cause is collection failure, not parsing failure

### 3. Timeline

- **14:00 UTC**: Normal log volume (~1200 logs/min from payment-service)
- **14:30 UTC**: Logrotate cron job runs, rotates `/var/log/payment-service.log` to `/var/log/payment-service.log.1`
- **14:32 UTC**: Agent can no longer tail original file path, log collection stops
- **14:35 UTC**: Monitor triggers: log volume dropped to zero
- **14:45 UTC**: Investigation begins; agent logs reveal file not found error

**Correlation**: Logrotate runs daily at 14:30; this is a recurring failure pattern that was previously undetected because the monitor was not configured.

### 4. Collection Analysis

**Current Agent Config** (`/etc/datadog-agent/conf.d/payment-service.d/conf.yaml`):
```yaml
logs:
  - type: file
    path: /var/log/payment-service.log  # Static path breaks after logrotate
    service: payment-service
    source: python
```

**Problem**: Static file path does not follow log rotation. After logrotate creates a new file, the agent continues trying to tail the old inode.

**Agent Status Output**:
```
$ sudo datadog-agent status
...
Logs Agent
==========
  payment-service
    Type: file
    Path: /var/log/payment-service.log
    Status: Error: no such file or directory
```

### 5. Pipeline Analysis

**Active Pipeline**: `[payment-service] Python Application Logs`

**Processor Chain**:
1. Grok Parser → **PASS** (pattern matches test sample)
2. Date Remapper → **PASS** (timestamp field exists and is valid ISO8601)
3. Status Remapper → **PASS** (level field maps to status correctly)
4. Service Remapper → **PASS** (service field extracted)
5. Trace ID Remapper → **N/A** (no trace_id in sample, non-blocking)

**Sample Log Through Pipeline**:
- **Input**: `INFO 2024-01-15T14:30:00Z payment-service Payment processed for order #12345`
- **After Grok**: `{level: INFO, timestamp: 2024-01-15T14:30:00Z, service: payment-service, message: Payment processed for order #12345}`
- **After Remappers**: `{status: info, @timestamp: 2024-01-15T14:30:00Z, service: payment-service, message: Payment processed for order #12345}`

**Conclusion**: Pipeline is functioning correctly; logs would be parsed properly if they reached Datadog.

### 6. Log-to-Metric Status

**Existing Metrics**:
- `payment.service.requests.count` — Count of all payment-service logs (currently zero due to collection failure)

**Missing Opportunities**:
- No error rate metric — should create `payment.service.errors.count` filtered by `status:error`
- No latency distribution — logs contain `duration_ms` field that could be extracted as distribution metric

**Cardinality Impact**: Current metric tags by `service` and `env` only (2 timeseries). Adding `endpoint` tag would increase to ~20 timeseries (acceptable).

### 7. Remediation

**Corrected Agent Config**:
```yaml
logs:
  - type: file
    path: /var/log/payment-service*.log  # Wildcard handles rotated files
    service: payment-service
    source: python
    tags:
      - env:prod
```

**Apply Fix**:
```bash
sudo vi /etc/datadog-agent/conf.d/payment-service.d/conf.yaml
# Update path to wildcard
sudo systemctl restart datadog-agent
```

**New Log-to-Metric Definitions**:
```bash
# Error count metric
curl -X POST "https://api.datadoghq.com/api/v2/logs/config/metrics" \
  -H "DD-API-KEY: ${DD_API_KEY}" \
  -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "type": "logs_metrics",
      "id": "payment.service.errors.count",
      "attributes": {
        "compute": {"aggregation_type": "count"},
        "filter": {"query": "service:payment-service status:error"},
        "group_by": [
          {"path": "env", "tag_name": "env"},
          {"path": "@endpoint", "tag_name": "endpoint"}
        ]
      }
    }
  }'
```

### 8. Validation Steps

**Agent Status Check**:
```bash
$ sudo datadog-agent status | grep -A5 payment-service
  payment-service
    Type: file
    Path: /var/log/payment-service*.log
    Status: OK
    Bytes Read: 125648
    Lines Sent: 1247
```

**Log Search Query**:
```
service:payment-service @timestamp:[now-5m TO now]
```
**Expected Output**: >100 log entries from the past 5 minutes

**Metric Explorer Query**:
```
payment.service.errors.count{*} by {service,endpoint}
```
**Expected Output**: Timeseries for each endpoint showing error counts (or zero if no errors)

### 9. Prevention

**Pipeline Health Monitor**:
```yaml
name: "[payment-service] Log parsing failure rate high"
type: log alert
query: "logs(\"service:payment-service\").rollup(\"count\").by(\"status\").last(\"15m\") > 100"
message: "Payment service logs are failing to parse. Check pipeline grok patterns."
```

**Volume Anomaly Detection**:
```yaml
name: "[payment-service] Log volume anomaly"
type: anomaly
query: "logs(\"service:payment-service\").rollup(\"count\").last(\"15m\")"
message: "Payment service log volume dropped unexpectedly. Check agent collection."
```

**Grok Pattern Regression Test**:
- Maintain `test_samples.txt` with real log examples
- Run `datadog-agent configcheck` in CI/CD before deploying agent config changes
- Alert on increased unparsed log count: `logs("service:payment-service -status:*").rollup("count").last("1h") > 50`

**Reference**: https://docs.datadoghq.com/logs/

---

## Constraints

- **Always test grok patterns** against real log samples before deploying — untested patterns cause silent parsing failures that drop log attributes and leave logs unsearchable by extracted fields
- **Use standard attributes** (`service`, `env`, `version`, `http.status_code`, `duration`) for correlation — non-standard attribute names break cross-product features (APM ↔ Logs ↔ Metrics correlation)
- **Apply exclusion filters** to reduce ingestion cost — health checks and debug logs in production can account for 60%+ of log volume with zero investigative value
- **Never parse PII without redaction** — use Sensitive Data Scanner or scrubbing rules before logs are indexed; logged PII creates compliance risk (GDPR, HIPAA, PCI-DSS violations)
- **Link logs to traces** by extracting and remapping `trace_id` — without this link, engineers cannot navigate from log to distributed trace during incident investigation
- **Prefer JSON logs** for structured extraction — JSON parsing is deterministic and does not break on format changes; grok parsing is brittle and requires maintenance when log formats evolve
- **Monitor pipeline performance** — track `datadog.logs_pipeline.events` and processor execution time; slow pipelines delay log availability in search and can cause agent-side buffering
- **Order processors correctly** — parsers must run before remappers; a status remapper on an unparsed log maps nothing because the `level` field doesn't exist yet
- **Create log-based metrics for alerting, not dashboards** — dashboard queries against indexed logs are slow (full-scan search) and expensive; log-based metrics are pre-aggregated and cost 1/100th as much
- Refer to Datadog Logs documentation: https://docs.datadoghq.com/logs/
- Refer to Datadog Log Processing API: https://docs.datadoghq.com/api/latest/logs-pipelines/
