package agents

import (
	"strings"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

// RoleClassifier determines which specialist agent should handle an alert
type RoleClassifier struct {
	// monitorTypeRules maps Datadog monitor types to agent roles
	monitorTypeRules map[string]AgentRole

	// tagRules maps tag prefixes to agent roles
	tagRules map[string]AgentRole

	// servicePatterns maps service name patterns to agent roles
	servicePatterns map[string]AgentRole

	// hostnamePatterns maps hostname patterns to agent roles
	hostnamePatterns map[string]AgentRole
}

// NewRoleClassifier creates a new classifier with default rules
func NewRoleClassifier() *RoleClassifier {
	return &RoleClassifier{
		monitorTypeRules: map[string]AgentRole{
			// APM and application monitors
			"apm":               RoleApplication,
			"trace-analytics":   RoleApplication,
			"rum":               RoleApplication,
			"error tracking":    RoleApplication,
			"profiling":         RoleApplication,

			// Infrastructure monitors
			"metric":            RoleInfrastructure,
			"host":              RoleInfrastructure,
			"process":           RoleInfrastructure,
			"integration":       RoleInfrastructure,
			"service check":     RoleInfrastructure,
			"outlier":           RoleInfrastructure,
			"forecast":          RoleInfrastructure,
			"anomaly":           RoleInfrastructure,

			// Database monitors
			"dbm":               RoleDatabase,
			"database":          RoleDatabase,

			// Log monitors
			"logs":              RoleLogs,
			"log":               RoleLogs,

			// Network monitors
			"synthetics":        RoleNetwork,
			"network":           RoleNetwork,
			"network performance": RoleNetwork,
		},

		tagRules: map[string]AgentRole{
			"monitor_type:apm":           RoleApplication,
			"monitor_type:dbm":           RoleDatabase,
			"monitor_type:logs":          RoleLogs,
			"monitor_type:network":       RoleNetwork,
			"monitor_type:infrastructure": RoleInfrastructure,
			"service_type:database":      RoleDatabase,
			"service_type:web":           RoleApplication,
			"service_type:api":           RoleApplication,
			"tier:database":              RoleDatabase,
			"tier:application":           RoleApplication,
			"tier:network":               RoleNetwork,
		},

		servicePatterns: map[string]AgentRole{
			// Database services
			"postgres":  RoleDatabase,
			"mysql":     RoleDatabase,
			"mongodb":   RoleDatabase,
			"redis":     RoleDatabase,
			"memcached": RoleDatabase,
			"cassandra": RoleDatabase,
			"db":        RoleDatabase,
			"database":  RoleDatabase,
			"rds":       RoleDatabase,
			"aurora":    RoleDatabase,
			"dynamo":    RoleDatabase,

			// Application services
			"api":       RoleApplication,
			"web":       RoleApplication,
			"frontend":  RoleApplication,
			"backend":   RoleApplication,
			"service":   RoleApplication,
			"app":       RoleApplication,
			"graphql":   RoleApplication,
			"rest":      RoleApplication,

			// Network services
			"nginx":     RoleNetwork,
			"haproxy":   RoleNetwork,
			"loadbalancer": RoleNetwork,
			"lb":        RoleNetwork,
			"cdn":       RoleNetwork,
			"gateway":   RoleNetwork,
			"proxy":     RoleNetwork,
			"dns":       RoleNetwork,

			// Infrastructure
			"kubernetes": RoleInfrastructure,
			"k8s":        RoleInfrastructure,
			"docker":     RoleInfrastructure,
			"container":  RoleInfrastructure,
			"ec2":        RoleInfrastructure,
			"lambda":     RoleInfrastructure,
			"ecs":        RoleInfrastructure,
		},

		hostnamePatterns: map[string]AgentRole{
			"db":      RoleDatabase,
			"mysql":   RoleDatabase,
			"postgres": RoleDatabase,
			"redis":   RoleDatabase,
			"mongo":   RoleDatabase,
			"web":     RoleApplication,
			"api":     RoleApplication,
			"app":     RoleApplication,
			"lb":      RoleNetwork,
			"proxy":   RoleNetwork,
			"nginx":   RoleNetwork,
		},
	}
}

// Classify determines the appropriate agent role for an alert event
func (c *RoleClassifier) Classify(event *types.AlertEvent) AgentRole {
	payload := event.Payload

	// 0. Check for Watchdog monitors first (highest priority, special Datadog feature)
	if IsWatchdog(payload.MonitorType, payload.MonitorName, payload.AlertTitle, payload.Tags) {
		return RoleWatchdog
	}

	// 1. Check explicit monitor type
	if role := c.classifyByMonitorType(payload.MonitorType); role != RoleGeneral {
		return role
	}

	// 2. Check tags for monitor_type or service_type hints
	if role := c.classifyByTags(payload.Tags); role != RoleGeneral {
		return role
	}

	// 3. Infer from service name patterns
	if role := c.classifyByService(payload.Service); role != RoleGeneral {
		return role
	}

	// 4. Infer from hostname patterns
	if role := c.classifyByHostname(payload.Hostname); role != RoleGeneral {
		return role
	}

	// 5. Default to infrastructure (most common)
	return RoleInfrastructure
}

// IsWatchdog determines if a monitor is a Datadog Watchdog anomaly detection monitor.
// Watchdog monitors can be identified by:
// - monitor_type containing "watchdog"
// - monitor name or alert title containing "watchdog"
// - tags containing "source:watchdog" or "monitor_type:watchdog"
func IsWatchdog(monitorType, monitorName, alertTitle string, tags []string) bool {
	mt := strings.ToLower(monitorType)
	if mt == "watchdog" || strings.Contains(mt, "watchdog") {
		return true
	}

	if strings.Contains(strings.ToLower(monitorName), "watchdog") {
		return true
	}

	if strings.Contains(strings.ToLower(alertTitle), "watchdog") {
		return true
	}

	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		if tagLower == "source:watchdog" || tagLower == "monitor_type:watchdog" || tagLower == "created_by:watchdog" {
			return true
		}
	}

	return false
}

// classifyByMonitorType checks the monitor type field
func (c *RoleClassifier) classifyByMonitorType(monitorType string) AgentRole {
	if monitorType == "" {
		return RoleGeneral
	}

	mt := strings.ToLower(monitorType)
	if role, ok := c.monitorTypeRules[mt]; ok {
		return role
	}

	// Check for partial matches
	for pattern, role := range c.monitorTypeRules {
		if strings.Contains(mt, pattern) {
			return role
		}
	}

	return RoleGeneral
}

// classifyByTags checks tags for role hints
func (c *RoleClassifier) classifyByTags(tags []string) AgentRole {
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)

		// Check exact tag matches
		if role, ok := c.tagRules[tagLower]; ok {
			return role
		}

		// Check for monitor_type: prefix
		if strings.HasPrefix(tagLower, "monitor_type:") {
			monitorType := strings.TrimPrefix(tagLower, "monitor_type:")
			if role, ok := c.monitorTypeRules[monitorType]; ok {
				return role
			}
		}
	}

	return RoleGeneral
}

// classifyByService checks service name for patterns
func (c *RoleClassifier) classifyByService(service string) AgentRole {
	if service == "" {
		return RoleGeneral
	}

	svc := strings.ToLower(service)
	for pattern, role := range c.servicePatterns {
		if strings.Contains(svc, pattern) {
			return role
		}
	}

	return RoleGeneral
}

// classifyByHostname checks hostname for patterns
func (c *RoleClassifier) classifyByHostname(hostname string) AgentRole {
	if hostname == "" {
		return RoleGeneral
	}

	host := strings.ToLower(hostname)
	for pattern, role := range c.hostnamePatterns {
		if strings.Contains(host, pattern) {
			return role
		}
	}

	return RoleGeneral
}

// containsAny checks if s contains any of the patterns
func containsAny(s string, patterns ...string) bool {
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
