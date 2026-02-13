# agentic_instructions.md

## Purpose
WebhookProcessor implementations for the webhook processing pipeline. Each processor handles a specific integration (desktop notifications, Slack, forwarding, downtimes, Claude agent RCA).

## Technology
Go, net/http, encoding/json, os (env vars)

## Contents
- `desktop_notify.go` -- DesktopNotifyProcessor: sends notifications to local desktop notification servers. Uses resolveTitle() for robust title extraction (MonitorName > AlertTitleCustom > AlertTitle > DetailedDescription first line > fallback)
- `downtime.go` -- DowntimeProcessor: creates auto-downtimes via Datadog API v2 when monitors recover
- `forwarding.go` -- ForwardingProcessor: forwards webhook payloads to configured URLs
- `slack.go` -- SlackProcessor: sends formatted Slack messages via incoming webhooks (template for new integrations)
- `claude_agent.go` -- ClaudeAgentProcessor: invokes Claude AI sidecar for RCA analysis (deprecated, replaced by agent orchestrator)

## Key Functions
- `NewDesktopNotifyProcessor() *DesktopNotifyProcessor` -- Multi-server support via NOTIFY_SERVER_URLS env
- `resolveTitle(p WebhookPayload) string` -- Extracts best available title from webhook payload fields. Handles watchdog alerts which arrive with empty standard fields by falling through MonitorName -> AlertTitleCustom -> AlertTitle -> DetailedDescription (first line) -> "Datadog Webhook"
- `NewDowntimeProcessor() *DowntimeProcessor` -- Default creds; `NewDowntimeProcessorWithAccounts()` for multi-account
- `NewForwardingProcessor() *ForwardingProcessor` -- Uses shared ForwardingClient with connection pooling
- `NewSlackProcessor() *SlackProcessor` -- Configured via SLACK_WEBHOOK_URL, SLACK_CHANNEL env vars
- `NewClaudeAgentProcessor() *ClaudeAgentProcessor` -- Configured via CLAUDE_AGENT_URL env var (default: localhost:9000)

## Data Types
- `CredentialProvider` -- interface: GetByID(id) (*accounts.Account, error), GetDefault() *accounts.Account
- All processors implement `webhooks.WebhookProcessor` interface: Name(), CanProcess(), Process()
- `slackMessage`, `slackAttachment`, `slackField` -- Slack API payload types
- `downtimeRequest`, `downtimeData`, `downtimeAttributes` -- Datadog downtime API v2 types
- `claudeAnalysisRequest`, `claudeAnalysisResponse` -- Claude sidecar API types

## Logging
Uses `log.Printf` with prefixes: `[NOTIFY-PROC]`, `[NOTIFY]`

## CRUD Entry Points
- **Create**: Copy `slack.go` as a template for new integrations (PagerDuty, Discord, Teams, etc.)
- **Read**: Import `processors.NewDesktopNotifyProcessor()` etc., register with orchestrator
- **Update**: Modify CanProcess() logic to change which events trigger the processor
- **Delete**: Remove processor file, unregister from orchestrator

## Style Guide
- Each processor is a single file implementing `webhooks.WebhookProcessor`
- Constructor reads env vars for configuration
- CanProcess() filters events; Process() performs the action
- Multi-account support via optional CredentialProvider interface
- Representative snippet:

```go
func (p *DesktopNotifyProcessor) Name() string {
	return "desktop_notify"
}

func (p *DesktopNotifyProcessor) CanProcess(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) bool {
	return true
}

func (p *DesktopNotifyProcessor) Process(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) webhooks.ProcessorResult {
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
	}
	err := p.sendNotification(event.Payload)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}
	result.Success = true
	result.Message = fmt.Sprintf("notification sent: %s", title)
	return result
}
```
