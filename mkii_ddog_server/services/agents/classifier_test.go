package agents

import (
	"testing"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

func TestRoleClassifier_ClassifyByMonitorType(t *testing.T) {
	classifier := NewRoleClassifier()

	tests := []struct {
		name        string
		monitorType string
		expected    AgentRole
	}{
		{"APM monitor", "apm", RoleApplication},
		{"Trace analytics", "trace-analytics", RoleApplication},
		{"RUM monitor", "rum", RoleApplication},
		{"Metric monitor", "metric", RoleInfrastructure},
		{"Host monitor", "host", RoleInfrastructure},
		{"Process monitor", "process", RoleInfrastructure},
		{"DBM monitor", "dbm", RoleDatabase},
		{"Database monitor", "database", RoleDatabase},
		{"Logs monitor", "logs", RoleLogs},
		{"Log monitor", "log", RoleLogs},
		{"Synthetics monitor", "synthetics", RoleNetwork},
		{"Network monitor", "network", RoleNetwork},
		{"Unknown type defaults to infrastructure", "unknown", RoleInfrastructure},
		{"Empty type defaults to infrastructure", "", RoleInfrastructure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					MonitorType: tt.monitorType,
				},
			}
			role := classifier.Classify(event)
			if role != tt.expected {
				t.Errorf("MonitorType %q: expected %s, got %s", tt.monitorType, tt.expected, role)
			}
		})
	}
}

func TestRoleClassifier_ClassifyByTags(t *testing.T) {
	classifier := NewRoleClassifier()

	tests := []struct {
		name     string
		tags     []string
		expected AgentRole
	}{
		{
			name:     "monitor_type:apm tag",
			tags:     []string{"env:prod", "monitor_type:apm"},
			expected: RoleApplication,
		},
		{
			name:     "monitor_type:dbm tag",
			tags:     []string{"monitor_type:dbm", "team:platform"},
			expected: RoleDatabase,
		},
		{
			name:     "monitor_type:logs tag",
			tags:     []string{"monitor_type:logs"},
			expected: RoleLogs,
		},
		{
			name:     "service_type:database tag",
			tags:     []string{"service_type:database"},
			expected: RoleDatabase,
		},
		{
			name:     "tier:application tag",
			tags:     []string{"tier:application"},
			expected: RoleApplication,
		},
		{
			name:     "No matching tags",
			tags:     []string{"env:prod", "team:platform"},
			expected: RoleInfrastructure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					Tags: tt.tags,
				},
			}
			role := classifier.Classify(event)
			if role != tt.expected {
				t.Errorf("Tags %v: expected %s, got %s", tt.tags, tt.expected, role)
			}
		})
	}
}

func TestRoleClassifier_ClassifyByService(t *testing.T) {
	classifier := NewRoleClassifier()

	tests := []struct {
		name     string
		service  string
		expected AgentRole
	}{
		{"PostgreSQL service", "user-postgres", RoleDatabase},
		{"MySQL service", "mysql-primary", RoleDatabase},
		{"Redis service", "redis-cache", RoleDatabase},
		{"MongoDB service", "mongodb-cluster", RoleDatabase},
		{"API service", "user-api", RoleApplication},
		{"Web service", "web-frontend", RoleApplication},
		{"Backend service", "order-backend", RoleApplication},
		{"GraphQL service", "graphql-server", RoleApplication},
		{"Nginx service", "nginx-lb", RoleNetwork},
		{"HAProxy service", "haproxy-edge", RoleNetwork},
		{"Gateway service", "edge-gateway", RoleNetwork},
		{"Kubernetes service", "kubernetes-scheduler", RoleInfrastructure},
		{"Unknown service", "some-random-thing", RoleInfrastructure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					Service: tt.service,
				},
			}
			role := classifier.Classify(event)
			if role != tt.expected {
				t.Errorf("Service %q: expected %s, got %s", tt.service, tt.expected, role)
			}
		})
	}
}

func TestRoleClassifier_ClassifyByHostname(t *testing.T) {
	classifier := NewRoleClassifier()

	tests := []struct {
		name     string
		hostname string
		expected AgentRole
	}{
		{"DB hostname", "db-primary-01", RoleDatabase},
		{"MySQL hostname", "mysql-replica-02", RoleDatabase},
		{"PostgreSQL hostname", "postgres-master", RoleDatabase},
		{"Redis hostname", "redis-cluster-01", RoleDatabase},
		{"Web hostname", "web-server-01", RoleApplication},
		{"API hostname", "api-server-02", RoleApplication},
		{"App hostname", "app-worker-03", RoleApplication},
		{"LB hostname", "lb-edge-01", RoleNetwork},
		{"Nginx hostname", "nginx-proxy-01", RoleNetwork},
		{"Generic hostname", "server-01", RoleInfrastructure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					Hostname: tt.hostname,
				},
			}
			role := classifier.Classify(event)
			if role != tt.expected {
				t.Errorf("Hostname %q: expected %s, got %s", tt.hostname, tt.expected, role)
			}
		})
	}
}

func TestRoleClassifier_Priority(t *testing.T) {
	classifier := NewRoleClassifier()

	// Monitor type takes precedence over tags
	t.Run("MonitorType beats tags", func(t *testing.T) {
		event := &types.AlertEvent{
			Payload: types.AlertPayload{
				MonitorType: "apm",                           // Application
				Tags:        []string{"monitor_type:logs"},   // Would be Logs
			},
		}
		role := classifier.Classify(event)
		if role != RoleApplication {
			t.Errorf("Expected Application (from MonitorType), got %s", role)
		}
	})

	// Tags take precedence over service
	t.Run("Tags beat service", func(t *testing.T) {
		event := &types.AlertEvent{
			Payload: types.AlertPayload{
				Tags:    []string{"monitor_type:dbm"}, // Database
				Service: "api-server",                 // Would be Application
			},
		}
		role := classifier.Classify(event)
		if role != RoleDatabase {
			t.Errorf("Expected Database (from Tags), got %s", role)
		}
	})

	// Service takes precedence over hostname
	t.Run("Service beats hostname", func(t *testing.T) {
		event := &types.AlertEvent{
			Payload: types.AlertPayload{
				Service:  "postgres-primary", // Database
				Hostname: "web-server-01",    // Would be Application
			},
		}
		role := classifier.Classify(event)
		if role != RoleDatabase {
			t.Errorf("Expected Database (from Service), got %s", role)
		}
	})
}

func TestRoleClassifier_CaseInsensitive(t *testing.T) {
	classifier := NewRoleClassifier()

	tests := []struct {
		name        string
		monitorType string
		expected    AgentRole
	}{
		{"Uppercase", "APM", RoleApplication},
		{"Mixed case", "Apm", RoleApplication},
		{"Lowercase", "apm", RoleApplication},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					MonitorType: tt.monitorType,
				},
			}
			role := classifier.Classify(event)
			if role != tt.expected {
				t.Errorf("MonitorType %q: expected %s, got %s", tt.monitorType, tt.expected, role)
			}
		})
	}
}

func TestNewRoleClassifier(t *testing.T) {
	classifier := NewRoleClassifier()

	if classifier == nil {
		t.Fatal("Classifier should not be nil")
	}
	if len(classifier.monitorTypeRules) == 0 {
		t.Error("monitorTypeRules should not be empty")
	}
	if len(classifier.tagRules) == 0 {
		t.Error("tagRules should not be empty")
	}
	if len(classifier.servicePatterns) == 0 {
		t.Error("servicePatterns should not be empty")
	}
	if len(classifier.hostnamePatterns) == 0 {
		t.Error("hostnamePatterns should not be empty")
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s        string
		patterns []string
		expected bool
	}{
		{"postgresql-primary", []string{"postgres", "mysql"}, true},
		{"mysql-replica", []string{"postgres", "mysql"}, true},
		{"redis-cache", []string{"postgres", "mysql"}, false},
		{"API-SERVER", []string{"api", "web"}, true},
		{"", []string{"api"}, false},
		{"api", []string{}, false},
	}

	for _, tt := range tests {
		result := containsAny(tt.s, tt.patterns...)
		if result != tt.expected {
			t.Errorf("containsAny(%q, %v): expected %v, got %v",
				tt.s, tt.patterns, tt.expected, result)
		}
	}
}
