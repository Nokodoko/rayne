package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"n0kos.com/frontend/templates"
)

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	mux := http.NewServeMux()

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Routes
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/", handleIndex)

	host := getEnv("SERVER_HOST", "0.0.0.0")
	port := getEnv("SERVER_PORT", "3000")
	addr := host + ":" + port

	log.Printf("Server starting on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	component := templates.Index()
	component.Render(r.Context(), w)
}
