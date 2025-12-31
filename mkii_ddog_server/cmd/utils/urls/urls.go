package urls

import "fmt"

// DOWNTIMES
var GetDowntimesUrl string = "https://api.ddog-gov.com/api/v2/downtime"

// AWS INTEGRATIONS
var GetAwsIntegrations string = "https://api.ddog-gov.com/api/v2/integration/aws/accounts"

// EVENTS
var GetEvents string = "https://api.ddog-gov.com/api/v2/events"

// HOSTS
var GetHosts string = "https://api.ddog-gov.com/api/v1/hosts"
var GetTotalActiveHosts string = "https://api.ddog-gov.com/api/v1/hosts/totals"
var GetAllHostTags string = "https://api.ddog-gov.com/api/v1/tags/hosts"

func GetHostTags(hostname string) string {
	return fmt.Sprintf("https://api.ddog-gov.com/api/v1/tags/hosts/%s", hostname)
}

// MONITORS
var SearchMontiors string = "https://api.ddog-gov.com/api/v1/monitor/search"

func ByMonitorId(monitor_id int) string {
	return fmt.Sprintf("https://api.ddog-gov.com/api/v1/monitor/%v", monitor_id)
}

// USERS
var GetUsers string = "https://api.datadoghq.com/api/v2/users"

// WEBHOOKS
var CreateWebhook string = "https://api.ddog-gov.com/api/v1/integration/webhooks/"

// IP RANGES
var GetIpRanges string = "https://ip-ranges.ddog-gov.com"

// METRICS
var GetServices string = "https://api.ddog-gov.com/api/v2/services/definitions"

// SERVICES
func GetMetrics(host string) string {
	return fmt.Sprintf("https://api.ddog-gov.com/api/v1/metrics?from={%s}", host)
}

// LOGS
var LogSearch string = "https://api.ddog-gov.com/api/v2/logs/events/search"

// SERVICE CATALOG
var ServiceDefinitions string = "https://api.ddog-gov.com/api/v2/services/definitions"

// RUM
var RUMApplications string = "https://api.ddog-gov.com/api/v2/rum/applications"

// SYNTHETICS
func GetSynthetic(publicID string) string {
	return fmt.Sprintf("https://api.ddog-gov.com/api/v1/synthetics/tests/%s", publicID)
}
