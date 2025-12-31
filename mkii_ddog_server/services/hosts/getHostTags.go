package hosts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/keys"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

// HostTagsResponse represents the response from the host tags endpoint
type HostTagsResponse struct {
	Tags []string `json:"tags"`
}

// GetHostTags fetches tags for a specific hostname
func GetHostTags(hostname string) ([]string, error) {
	body := bytes.NewBufferString("")

	req, err := http.NewRequest("GET", urls.GetHostTags(hostname), body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var tagsResponse HostTagsResponse
	if err = json.Unmarshal(bodyBytes, &tagsResponse); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	return tagsResponse.Tags, nil
}

// GetHostTagsHandler is an HTTP handler for getting host tags
func GetHostTagsHandler(w http.ResponseWriter, r *http.Request, hostname string) (int, any) {
	tags, err := GetHostTags(hostname)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, map[string][]string{"tags": tags}
}

// GetAllHostsTags fetches tags for all hosts
func GetAllHostsTags(w http.ResponseWriter, r *http.Request) (int, any) {
	hostnames := GetHostsHelper(w, r)
	if hostnames == nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to fetch hosts"}
	}

	result := make(map[string][]string)
	for _, hostname := range hostnames {
		tags, err := GetHostTags(hostname)
		if err != nil {
			result[hostname] = []string{"error: " + err.Error()}
			continue
		}
		result[hostname] = tags
	}

	return http.StatusOK, result
}
