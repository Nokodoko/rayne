package monitors

import (
	"net/http"
	"strconv"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

// ListMonitors retrieves all monitors with pagination
func ListMonitors(w http.ResponseWriter, r *http.Request) (int, any) {
	page := 0
	perPage := 30

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed >= 0 {
			page = parsed
		}
	}

	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}

	url := urls.SearchMontiors + "?page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(perPage)

	result, status, err := requests.Get[MonitorSearchResponse](w, r, url)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, MonitorListResponse{
		Monitors:   result.Monitors,
		Page:       result.Metadata.Page,
		PageCount:  result.Metadata.PageCount,
		PerPage:    result.Metadata.PerPage,
		TotalCount: result.Metadata.Total,
	}
}

// GetMonitorPageCount retrieves pagination metadata
func GetMonitorPageCount(w http.ResponseWriter, r *http.Request) (int, any) {
	result, status, err := requests.Get[MonitorSearchResponse](w, r, urls.SearchMontiors)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result.Metadata
}

// GetTriggeredMonitors retrieves monitors that are currently triggered (in Alert status)
func GetTriggeredMonitors(w http.ResponseWriter, r *http.Request) (int, any) {
	// Fetch all monitors with pagination
	allMonitors := []Monitor{}
	page := 0
	perPage := 100

	for {
		url := urls.SearchMontiors + "?page=" + strconv.Itoa(page) + "&per_page=" + strconv.Itoa(perPage)
		result, status, err := requests.Get[MonitorSearchResponse](w, r, url)
		if err != nil {
			return http.StatusInternalServerError, map[string]string{"error": err.Error()}
		}
		if status != http.StatusOK {
			return status, result
		}

		allMonitors = append(allMonitors, result.Monitors...)

		if page >= result.Metadata.PageCount-1 {
			break
		}
		page++
	}

	// Filter for triggered monitors (status = Alert or Warn)
	triggered := []Monitor{}
	for _, m := range allMonitors {
		if m.Status == "Alert" || m.Status == "Warn" || m.OverallState == "Alert" || m.OverallState == "Warn" {
			triggered = append(triggered, m)
		}
	}

	return http.StatusOK, TriggeredMonitorsResponse{
		Monitors: triggered,
		Count:    len(triggered),
	}
}

// GetMonitorByID retrieves a specific monitor by ID
func GetMonitorByID(w http.ResponseWriter, r *http.Request, idStr string) (int, any) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid monitor ID"}
	}

	url := urls.ByMonitorId(id)
	result, status, err := requests.Get[Monitor](w, r, url)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}

// GetMonitorIDs retrieves just the monitor IDs and names
func GetMonitorIDs(w http.ResponseWriter, r *http.Request) (int, any) {
	result, status, err := requests.Get[MonitorSearchResponse](w, r, urls.SearchMontiors)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	type MonitorIDInfo struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	ids := make([]MonitorIDInfo, len(result.Monitors))
	for i, m := range result.Monitors {
		ids[i] = MonitorIDInfo{
			ID:     m.ID,
			Name:   m.Name,
			Status: m.Status,
		}
	}

	return status, map[string]any{
		"monitors": ids,
		"count":    len(ids),
	}
}
