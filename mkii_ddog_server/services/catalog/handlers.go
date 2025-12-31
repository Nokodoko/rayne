package catalog

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

// ListServices retrieves all service definitions from the catalog
func ListServices(w http.ResponseWriter, r *http.Request) (int, any) {
	result, status, err := requests.Get[ServiceListResponse](w, r, urls.ServiceDefinitions)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}

// CreateServiceDefinition creates a new service definition in the catalog
func CreateServiceDefinition(w http.ResponseWriter, r *http.Request) (int, any) {
	var req SimpleServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()}
	}

	if req.Name == "" {
		return http.StatusBadRequest, map[string]string{"error": "name is required"}
	}

	if req.Language == "" {
		req.Language = "unknown"
	}

	if req.Type == "" {
		req.Type = "web"
	}

	// Build tags from the request fields
	tags := []string{}
	if req.Platform != "" {
		tags = append(tags, fmt.Sprintf("platform:%s", req.Platform))
	}
	tags = append(tags, fmt.Sprintf("serverless:%t", req.Serverless))
	if req.Host != "" {
		tags = append(tags, fmt.Sprintf("host:%s", req.Host))
	}
	if req.URL != "" {
		tags = append(tags, fmt.Sprintf("url:%s", req.URL))
	}
	if req.Env != "" {
		tags = append(tags, fmt.Sprintf("env:%s", req.Env))
	}
	tags = append(tags, fmt.Sprintf("apm:%t", req.APM))
	tags = append(tags, fmt.Sprintf("rum:%t", req.RUM))
	tags = append(tags, fmt.Sprintf("active:%t", req.Active))
	if req.Version != "" {
		tags = append(tags, fmt.Sprintf("version:%s", req.Version))
	}
	if req.Owner != "" {
		tags = append(tags, fmt.Sprintf("owner:%s", req.Owner))
	}
	if req.CostCenter != "" {
		tags = append(tags, fmt.Sprintf("cost_center:%s", req.CostCenter))
	}
	if req.Department != "" {
		tags = append(tags, fmt.Sprintf("department:%s", req.Department))
	}

	// Build the Datadog API request
	ddReq := ServiceDefinitionRequest{
		SchemaVersion: "v2.2",
		DDService:     req.Name,
		Type:          req.Type,
		Languages:     []string{req.Language},
		Tags:          tags,
		Team:          req.Team,
	}

	result, status, err := requests.Post[ServiceDefinitionResponse](w, r, urls.ServiceDefinitions, ddReq)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}

// CreateServiceDefinitionAdvanced allows full control over the service definition
func CreateServiceDefinitionAdvanced(w http.ResponseWriter, r *http.Request) (int, any) {
	var req ServiceDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()}
	}

	if req.DDService == "" {
		return http.StatusBadRequest, map[string]string{"error": "dd-service is required"}
	}

	if req.SchemaVersion == "" {
		req.SchemaVersion = "v2.2"
	}

	result, status, err := requests.Post[ServiceDefinitionResponse](w, r, urls.ServiceDefinitions, req)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}
