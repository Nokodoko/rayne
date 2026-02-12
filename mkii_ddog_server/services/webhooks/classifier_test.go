package webhooks

import (
	"testing"
)

func TestIsWatchdogMonitor(t *testing.T) {
	tests := []struct {
		name     string
		payload  WebhookPayload
		expected bool
	}{
		{
			name: "watchdog monitor type",
			payload: WebhookPayload{
				MonitorType: "watchdog",
				MonitorName: "CPU anomaly",
			},
			expected: true,
		},
		{
			name: "watchdog monitor type uppercase",
			payload: WebhookPayload{
				MonitorType: "Watchdog",
				MonitorName: "Memory spike",
			},
			expected: true,
		},
		{
			name: "watchdog in monitor name",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "[Watchdog] Anomaly on host-01",
			},
			expected: true,
		},
		{
			name: "watchdog in alert title",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "Some monitor",
				AlertTitle:  "Watchdog detected anomaly in CPU",
			},
			expected: true,
		},
		{
			name: "watchdog in custom alert title",
			payload: WebhookPayload{
				MonitorType:      "metric",
				MonitorName:      "Some monitor",
				AlertTitleCustom: "Watchdog: Anomalous behavior",
			},
			expected: true,
		},
		{
			name: "source:watchdog tag",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "Some monitor",
				Tags:        []string{"env:prod", "source:watchdog"},
			},
			expected: true,
		},
		{
			name: "monitor_type:watchdog tag",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "Some monitor",
				Tags:        []string{"monitor_type:watchdog"},
			},
			expected: true,
		},
		{
			name: "created_by:watchdog tag",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "Some monitor",
				Tags:        []string{"created_by:watchdog", "env:staging"},
			},
			expected: true,
		},
		{
			name: "standard metric monitor - not watchdog",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "CPU usage on web-01",
				Tags:        []string{"env:prod", "service:web"},
			},
			expected: false,
		},
		{
			name: "apm monitor - not watchdog",
			payload: WebhookPayload{
				MonitorType: "apm",
				MonitorName: "High error rate",
			},
			expected: false,
		},
		{
			name: "empty payload - not watchdog",
			payload: WebhookPayload{},
			expected: false,
		},
		{
			name: "watchdog partial match in monitor type",
			payload: WebhookPayload{
				MonitorType: "watchdog_anomaly",
				MonitorName: "Latency spike",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWatchdogMonitor(&tt.payload)
			if result != tt.expected {
				t.Errorf("IsWatchdogMonitor() = %v, want %v for payload: type=%q, name=%q, tags=%v",
					result, tt.expected, tt.payload.MonitorType, tt.payload.MonitorName, tt.payload.Tags)
			}
		})
	}
}

func TestClassifyMonitorType(t *testing.T) {
	tests := []struct {
		name     string
		payload  WebhookPayload
		expected string
	}{
		{
			name: "watchdog monitor returns watchdog",
			payload: WebhookPayload{
				MonitorType: "watchdog",
				MonitorName: "Anomaly detected",
			},
			expected: "watchdog",
		},
		{
			name: "watchdog by name returns watchdog",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "[Watchdog] CPU anomaly",
			},
			expected: "watchdog",
		},
		{
			name: "standard monitor returns empty",
			payload: WebhookPayload{
				MonitorType: "metric",
				MonitorName: "CPU usage high",
			},
			expected: "",
		},
		{
			name: "apm monitor returns empty",
			payload: WebhookPayload{
				MonitorType: "apm",
				MonitorName: "Service latency",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyMonitorType(&tt.payload)
			if result != tt.expected {
				t.Errorf("ClassifyMonitorType() = %q, want %q", result, tt.expected)
			}
		})
	}
}
