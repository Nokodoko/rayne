package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/keys"
)

// DowntimeService handles automatic downtime creation
type DowntimeService struct {
	apiURL string
}

// NewDowntimeService creates a new downtime service
func NewDowntimeService() *DowntimeService {
	return &DowntimeService{
		apiURL: "https://api.datadoghq.com/api/v2/downtime",
	}
}

// DowntimeRequest represents the Datadog downtime API request
type DowntimeRequest struct {
	Data DowntimeData `json:"data"`
}

type DowntimeData struct {
	Type       string             `json:"type"`
	Attributes DowntimeAttributes `json:"attributes"`
}

type DowntimeAttributes struct {
	Message          string           `json:"message,omitempty"`
	MonitorIdentifier MonitorIdentifier `json:"monitor_identifier"`
	Scope            string           `json:"scope"`
	Schedule         DowntimeSchedule `json:"schedule"`
}

type MonitorIdentifier struct {
	MonitorID int64 `json:"monitor_id"`
}

type DowntimeSchedule struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// DowntimeResponse represents the response from creating a downtime
type DowntimeResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Message   string `json:"message"`
			MonitorID int64  `json:"monitor_id"`
			Scope     string `json:"scope"`
			Status    string `json:"status"`
		} `json:"attributes"`
	} `json:"data"`
}

// CreateForMonitor creates a downtime for a specific monitor
func (d *DowntimeService) CreateForMonitor(monitorID int64, scope string, durationMinutes int) error {
	now := time.Now().UTC()
	end := now.Add(time.Duration(durationMinutes) * time.Minute)

	// Convert scope from comma-separated to Datadog format
	scopeFormatted := formatScope(scope)

	request := DowntimeRequest{
		Data: DowntimeData{
			Type: "downtime",
			Attributes: DowntimeAttributes{
				Message: fmt.Sprintf("Auto-created downtime after monitor recovery (ID: %d)", monitorID),
				MonitorIdentifier: MonitorIdentifier{
					MonitorID: monitorID,
				},
				Scope: scopeFormatted,
				Schedule: DowntimeSchedule{
					Start: now.Format(time.RFC3339),
					End:   end.Format(time.RFC3339),
				},
			},
		},
	}

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal downtime request: %v", err)
	}

	req, err := http.NewRequest("POST", d.apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("downtime creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// formatScope converts a comma-separated scope to Datadog format
// e.g., "platform:aws,team:backend" -> "platform:aws AND team:backend"
func formatScope(scope string) string {
	if scope == "" {
		return "*"
	}

	// Replace commas with AND
	parts := strings.Split(scope, ",")
	var cleanParts []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}

	if len(cleanParts) == 0 {
		return "*"
	}

	return strings.Join(cleanParts, " AND ")
}

// GetActiveDowntimes retrieves currently active downtimes
func (d *DowntimeService) GetActiveDowntimes() ([]map[string]interface{}, error) {
	req, err := http.NewRequest("GET", d.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result.Data, nil
}

// CheckDuplicateDowntime checks if a downtime already exists for the monitor
func (d *DowntimeService) CheckDuplicateDowntime(monitorID int64, scope string) (bool, error) {
	downtimes, err := d.GetActiveDowntimes()
	if err != nil {
		return false, err
	}

	for _, dt := range downtimes {
		if attrs, ok := dt["attributes"].(map[string]interface{}); ok {
			if mid, ok := attrs["monitor_id"].(float64); ok && int64(mid) == monitorID {
				if dtScope, ok := attrs["scope"].(string); ok && dtScope == formatScope(scope) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
