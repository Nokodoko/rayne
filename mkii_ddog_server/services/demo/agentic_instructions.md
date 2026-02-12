# agentic_instructions.md

## Purpose
Demo data generation for development and testing. Seeds fake webhook events, RUM sessions/visitors/events, and generates sample monitors. Also provides intentional error generation for APM testing.

## Technology
Go, math/rand, github.com/google/uuid

## Contents
- `handler.go` -- HTTP handlers for seeding data and generating errors
- `generators.go` -- Random data generators for webhooks, RUM sessions, and monitors

## Key Functions
- `NewHandler(webhookStorage, rumStorage) *Handler` -- Creates demo handler with both storage backends
- `(h *Handler) SeedWebhookEvents(w, r) (int, any)` -- Generates N fake webhook events (default: 50, max: 500)
- `(h *Handler) SeedRUMData(w, r) (int, any)` -- Generates N fake RUM sessions with events (default: 100, max: 1000)
- `(h *Handler) SeedAllData(w, r) (int, any)` -- Seeds both webhook and RUM data in one call
- `(h *Handler) GenerateError(w, r) (int, any)` -- Returns intentional HTTP errors for APM testing (server, timeout, database, upstream, validation, auth, ratelimit, etc.)
- `GenerateWebhookPayload() webhooks.WebhookPayload` -- Creates fake webhook payload with realistic data
- `GenerateRUMSession() (Visitor, Session, []RUMEvent)` -- Creates visitor, session, and events
- `GenerateMonitorAlert() map[string]interface{}` -- Creates fake monitor alert data

## Data Types
- `Handler` -- struct: webhookStorage, rumStorage
- Seed data variables: services, alertStatuses, monitorNames, hostnames, scopes, pagePaths, userAgents, referrers

## Logging
None (errors returned in response)

## CRUD Entry Points
- **Create**: Call seed endpoints via POST /v1/demo/seed/webhooks, /v1/demo/seed/rum, /v1/demo/seed/all
- **Read**: GET /v1/demo/status returns current demo data stats
- **Update**: Modify generators.go seed data arrays for different fake data
- **Delete**: N/A (seeded data is real data in the database)

## Style Guide
- Query param configuration: `?count=50`, `?webhooks=50&rum=100`
- Error limiting: `errors[:min(5, len(errors))]` to prevent oversized responses
- Representative snippet:

```go
func GenerateWebhookPayload() webhooks.WebhookPayload {
	now := time.Now()
	alertStatus := alertStatuses[rand.Intn(len(alertStatuses))]

	return webhooks.WebhookPayload{
		AlertID:     rand.Int63n(1000000) + 1,
		AlertTitle:  fmt.Sprintf("[%s] %s", alertStatus, monitorNames[rand.Intn(len(monitorNames))]),
		AlertStatus: alertStatus,
		MonitorID:   rand.Int63n(10000) + 1,
		MonitorName: monitorNames[rand.Intn(len(monitorNames))],
		Hostname:    hostnames[rand.Intn(len(hostnames))],
		Service:     services[rand.Intn(len(services))],
	}
}
```
