# agentic_instructions.md

## Purpose
Shared data types used across services -- user types, alert payload types, and alert event types.

## Technology
Go structs with JSON tags

## Contents
- `types.go` -- RegisterUserPayload, User struct, UserStorage interface
- `alert.go` -- AlertPayload struct (standard Datadog + custom Terraform fields), AlertEvent struct (stored alert with metadata)

## Key Functions
None (pure type definitions)

## Data Types
- `RegisterUserPayload` -- Name string, UUID int
- `User` -- Name string, UUID int
- `UserStorage` -- interface: GetUserbyUUID(uuid int), CreateUser(name string, uuid int)
- `AlertPayload` -- 30+ fields covering standard Datadog webhook fields (AlertID, MonitorID, Hostname, Service, Tags, etc.) and custom fields (ALERT_STATE, APPLICATION_TEAM, IMPACT, etc.)
- `AlertEvent` -- ID int64, Payload AlertPayload, ReceivedAt time.Time, ProcessedAt, Status, ForwardedTo, Error

## Logging
None

## CRUD Entry Points
- **Create**: Add new struct or interface in a new or existing file
- **Read**: Import `github.com/Nokodoko/mkii_ddog_server/cmd/types`
- **Update**: Modify struct fields (update JSON tags accordingly)
- **Delete**: Remove struct definition (check for import usages first)

## Style Guide
- PascalCase for exported fields with `json:"snake_case"` tags
- Custom Terraform fields use SCREAMING_SNAKE_CASE JSON tags matching Datadog webhook template variables
- Representative snippet:

```go
type AlertPayload struct {
	AlertID       int64    `json:"alert_id"`
	AlertTitle    string   `json:"alert_title"`
	AlertStatus   string   `json:"alert_status"`
	MonitorID     int64    `json:"monitor_id"`
	Tags          []string `json:"tags"`
	Hostname      string   `json:"hostname"`
	// Custom fields from Terraform webhook config
	AlertState          string `json:"ALERT_STATE"`
	ApplicationTeam     string `json:"APPLICATION_TEAM"`
}
```
