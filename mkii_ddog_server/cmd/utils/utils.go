package utils

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func Endpoint(router *http.ServeMux, method string, path string, endpt func(w http.ResponseWriter, r *http.Request) (int, any)) {
	// Use Go 1.22+ method routing pattern: "METHOD /path"
	pattern := method + " " + path
	router.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status, resp := endpt(w, r)
		if resp != nil {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(status)
		}
	})
}

func EndpointWithPathParams(router *http.ServeMux, method string, path string, val string, endpt func(w http.ResponseWriter, r *http.Request, pv string) (int, any)) {
	// Use Go 1.22+ method routing pattern: "METHOD /path"
	pattern := method + " " + path
	router.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pathValue := r.PathValue(val)
		status, resp := endpt(w, r, pathValue)
		if resp != nil {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(status)
		}
	})
}

func GetEnv(key string, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	return value
}

func ParseJson[T any](r *http.Request, payload *T) (int, any, error) {
	zero := new(*T)
	if r.Body == nil {
		log.Println("Empty request body")
		return http.StatusBadRequest, zero, nil
	}
	err := json.NewDecoder(r.Body).Decode(payload)
	if err != nil {
		return http.StatusBadRequest, err, nil
	}
	return http.StatusOK, zero, nil
}

func WriteJson[T any](w http.ResponseWriter, status int, data T) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, v error) {
	WriteJson(w, status, map[string]string{"Error:": v.Error()})
	return
}
