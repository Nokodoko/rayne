package requests

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/keys"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

// tracedClient is a shared HTTP client with Datadog APM tracing enabled
// for all outgoing requests to the Datadog API
var tracedClient = httptrace.WrapClient(&http.Client{
	Timeout: 30 * time.Second,
}, httptrace.RTWithResourceNamer(func(req *http.Request) string {
	return req.Method + " " + req.URL.Path
}))

func Get[T any](w http.ResponseWriter, r *http.Request, url string) (T, int, error) {
	var parsedResponse T
	zero := *new(T) // HACK:work around to return a nil value for a generic type -- contstruct a new type T with zero value

	body := bytes.NewBufferString("")
	req, err := http.NewRequest("GET", url, body)
	if err != nil {
		log.Fatalf("error: %v", err)
		return zero, http.StatusInternalServerError, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	// Use traced client for APM visibility into outgoing Datadog API calls
	response, err := tracedClient.Do(req)
	if err != nil {
		log.Printf("Error making request: %v", err)
		return zero, http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	// if err := utils.ParseJson(response.Body, parsedResponse); err != nil {
	// 	utils.WriteError(w, http.StatusBadRequest, err)
	// }
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading bytes: %v", err)
		return zero, http.StatusInternalServerError, err
	}

	err = json.Unmarshal(bodyBytes, &parsedResponse)
	if err != nil {
		log.Printf("Error parsing json: %v", err)
		return zero, http.StatusInternalServerError, err
	}
	return parsedResponse, http.StatusOK, nil
}

// Post sends a POST request to the Datadog API
func Post[T any](w http.ResponseWriter, r *http.Request, url string, payload interface{}) (T, int, error) {
	var parsedResponse T
	zero := *new(T)

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	// Use traced client for APM visibility into outgoing Datadog API calls
	response, err := tracedClient.Do(req)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	if err = json.Unmarshal(bodyBytes, &parsedResponse); err != nil {
		return zero, http.StatusInternalServerError, err
	}

	return parsedResponse, response.StatusCode, nil
}

// Put sends a PUT request to the Datadog API
func Put[T any](w http.ResponseWriter, r *http.Request, url string, payload interface{}) (T, int, error) {
	var parsedResponse T
	zero := *new(T)

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	// Use traced client for APM visibility into outgoing Datadog API calls
	response, err := tracedClient.Do(req)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	if err = json.Unmarshal(bodyBytes, &parsedResponse); err != nil {
		return zero, http.StatusInternalServerError, err
	}

	return parsedResponse, response.StatusCode, nil
}

// Delete sends a DELETE request to the Datadog API
func Delete[T any](w http.ResponseWriter, r *http.Request, url string) (T, int, error) {
	var parsedResponse T
	zero := *new(T)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", keys.Api())
	req.Header.Set("DD-APPLICATION-KEY", keys.App())

	// Use traced client for APM visibility into outgoing Datadog API calls
	response, err := tracedClient.Do(req)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return zero, http.StatusInternalServerError, err
	}

	// DELETE may return empty body
	if len(bodyBytes) > 0 {
		if err = json.Unmarshal(bodyBytes, &parsedResponse); err != nil {
			return zero, http.StatusInternalServerError, err
		}
	}

	return parsedResponse, response.StatusCode, nil
}
