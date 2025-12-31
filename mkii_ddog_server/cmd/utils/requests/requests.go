package requests

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	// "github.com/!data!dog/dd-trace-go/ddtrace/tracer"
	// "github.com/Datadog/dd-trace-go/v2/ddrace/tracer"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/keys"
	// "github.com/n0ko/ddog_test_server/utils"
)

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

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error: %v", err)
		return zero, http.StatusInternalServerError, err
	}

	// span := tracer.StartSpan("web.request", tracer.ResourceName("/get"))
	// defer span.Finish()
	// log.Printf("Span Entry %v", span)

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

	client := &http.Client{}
	response, err := client.Do(req)
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

	client := &http.Client{}
	response, err := client.Do(req)
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

	client := &http.Client{}
	response, err := client.Do(req)
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
