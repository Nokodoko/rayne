# agentic_instructions.md

## Purpose
Datadog API URL constants and URL builder functions for all supported endpoints.

## Technology
Go, fmt.Sprintf

## Contents
- `urls.go` -- Package-level URL variables and helper functions for parameterized URLs

## Key Functions
- `GetHostTags(hostname string) string` -- Returns host tags URL
- `ByMonitorId(monitor_id int) string` -- Returns monitor-by-ID URL
- `GetMetrics(host string) string` -- Returns metrics URL for a host
- `GetSynthetic(publicID string) string` -- Returns synthetic test URL

## Data Types
Package-level string variables:
- `GetDowntimesUrl`, `GetAwsIntegrations`, `GetEvents`, `GetHosts`, `GetTotalActiveHosts`, `GetAllHostTags`, `SearchMontiors`, `GetUsers`, `CreateWebhook`, `GetIpRanges`, `GetServices`, `LogSearch`, `ServiceDefinitions`, `RUMApplications`

## Logging
None

## CRUD Entry Points
- **Create**: Add new URL constant or builder function
- **Read**: Import and reference `urls.GetHosts` etc.
- **Update**: Change base URL or API version in the string
- **Delete**: Remove unused URL variables

## Style Guide
- Package-level `var` for static URLs, functions for parameterized URLs
- All URLs target `https://api.datadoghq.com` (US Commercial)
- Representative snippet:

```go
var GetHosts string = "https://api.datadoghq.com/api/v1/hosts"

func GetHostTags(hostname string) string {
	return fmt.Sprintf("https://api.datadoghq.com/api/v1/tags/hosts/%s", hostname)
}
```
