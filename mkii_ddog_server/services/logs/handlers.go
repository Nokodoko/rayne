package logs

import (
	"encoding/json"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

// SearchLogs searches logs based on provided filter criteria
func SearchLogs(w http.ResponseWriter, r *http.Request) (int, any) {
	var req SimpleSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()}
	}

	if req.Query == "" {
		req.Query = "*"
	}
	if req.Limit == 0 {
		req.Limit = 50
	}
	if req.Limit > 1000 {
		req.Limit = 1000
	}

	// Build the Datadog API request
	searchReq := LogSearchRequest{
		Filter: LogFilter{
			Query:   req.Query,
			Indexes: []string{"main"},
			From:    req.From,
			To:      req.To,
		},
		Sort: "timestamp",
		Page: LogPage{
			Limit: req.Limit,
		},
	}

	result, status, err := requests.Post[LogSearchResponse](w, r, urls.LogSearch, searchReq)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}

// SearchLogsAdvanced allows full control over the search request
func SearchLogsAdvanced(w http.ResponseWriter, r *http.Request) (int, any) {
	var req LogSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()}
	}

	if req.Filter.Query == "" {
		req.Filter.Query = "*"
	}
	if req.Page.Limit == 0 {
		req.Page.Limit = 50
	}

	result, status, err := requests.Post[LogSearchResponse](w, r, urls.LogSearch, req)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}
