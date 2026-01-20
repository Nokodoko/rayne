package demo

import (
	"net/http"
	"strconv"

	"github.com/Nokodoko/mkii_ddog_server/services/rum"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// Handler handles demo data generation endpoints
type Handler struct {
	webhookStorage *webhooks.Storage
	rumStorage     *rum.Storage
}

// NewHandler creates a new demo handler
func NewHandler(webhookStorage *webhooks.Storage, rumStorage *rum.Storage) *Handler {
	return &Handler{
		webhookStorage: webhookStorage,
		rumStorage:     rumStorage,
	}
}

// SeedWebhookEvents generates and stores fake webhook events
func (h *Handler) SeedWebhookEvents(w http.ResponseWriter, r *http.Request) (int, any) {
	count := 50 // default
	if c := r.URL.Query().Get("count"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed > 0 && parsed <= 500 {
			count = parsed
		}
	}

	created := 0
	var errors []string

	for i := 0; i < count; i++ {
		payload := GenerateWebhookPayload()
		_, err := h.webhookStorage.StoreEvent(payload)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		created++
	}

	response := map[string]interface{}{
		"requested": count,
		"created":   created,
		"message":   "Webhook events seeded successfully",
	}

	if len(errors) > 0 {
		response["errors"] = errors[:min(5, len(errors))] // Limit error messages
		response["error_count"] = len(errors)
	}

	return http.StatusCreated, response
}

// SeedRUMData generates and stores fake RUM sessions and events
func (h *Handler) SeedRUMData(w http.ResponseWriter, r *http.Request) (int, any) {
	count := 100 // default sessions
	if c := r.URL.Query().Get("count"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed > 0 && parsed <= 1000 {
			count = parsed
		}
	}

	sessionsCreated := 0
	eventsCreated := 0
	var errors []string

	for i := 0; i < count; i++ {
		visitor, session, events := GenerateRUMSession()

		// Create visitor
		if err := h.rumStorage.CreateVisitor(visitor.UUID, visitor.UserAgent, visitor.IPHash); err != nil {
			errors = append(errors, "visitor: "+err.Error())
			continue
		}

		// Create session
		if err := h.rumStorage.CreateSession(session.VisitorUUID, session.SessionID, session.Referrer, session.EntryPage, session.UserAgent); err != nil {
			errors = append(errors, "session: "+err.Error())
			continue
		}

		sessionsCreated++

		// Create events
		for _, event := range events {
			if err := h.rumStorage.StoreEvent(event); err != nil {
				errors = append(errors, "event: "+err.Error())
				continue
			}
			eventsCreated++
		}

		// Update session with final stats
		h.rumStorage.UpdateSession(session.SessionID, session.PageViews, session.DurationMs)
	}

	response := map[string]interface{}{
		"requested_sessions": count,
		"sessions_created":   sessionsCreated,
		"events_created":     eventsCreated,
		"message":            "RUM data seeded successfully",
	}

	if len(errors) > 0 {
		response["errors"] = errors[:min(5, len(errors))]
		response["error_count"] = len(errors)
	}

	return http.StatusCreated, response
}

// SeedAllData seeds both webhook and RUM data
func (h *Handler) SeedAllData(w http.ResponseWriter, r *http.Request) (int, any) {
	webhookCount := 50
	rumCount := 100

	if c := r.URL.Query().Get("webhooks"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed > 0 && parsed <= 500 {
			webhookCount = parsed
		}
	}

	if c := r.URL.Query().Get("rum"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed > 0 && parsed <= 1000 {
			rumCount = parsed
		}
	}

	// Seed webhooks
	webhooksCreated := 0
	for i := 0; i < webhookCount; i++ {
		payload := GenerateWebhookPayload()
		if _, err := h.webhookStorage.StoreEvent(payload); err == nil {
			webhooksCreated++
		}
	}

	// Seed RUM
	sessionsCreated := 0
	eventsCreated := 0
	for i := 0; i < rumCount; i++ {
		visitor, session, events := GenerateRUMSession()

		if err := h.rumStorage.CreateVisitor(visitor.UUID, visitor.UserAgent, visitor.IPHash); err != nil {
			continue
		}

		if err := h.rumStorage.CreateSession(session.VisitorUUID, session.SessionID, session.Referrer, session.EntryPage, session.UserAgent); err != nil {
			continue
		}

		sessionsCreated++

		for _, event := range events {
			if err := h.rumStorage.StoreEvent(event); err == nil {
				eventsCreated++
			}
		}

		h.rumStorage.UpdateSession(session.SessionID, session.PageViews, session.DurationMs)
	}

	return http.StatusCreated, map[string]interface{}{
		"webhooks_created":  webhooksCreated,
		"sessions_created":  sessionsCreated,
		"events_created":    eventsCreated,
		"message":           "Demo data seeded successfully",
	}
}

// GenerateSampleMonitors generates fake monitor data (read-only, doesn't store)
func (h *Handler) GenerateSampleMonitors(w http.ResponseWriter, r *http.Request) (int, any) {
	count := 20
	if c := r.URL.Query().Get("count"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed > 0 && parsed <= 100 {
			count = parsed
		}
	}

	var monitors []map[string]interface{}
	for i := 0; i < count; i++ {
		monitors = append(monitors, GenerateMonitorAlert())
	}

	return http.StatusOK, map[string]interface{}{
		"count":    count,
		"monitors": monitors,
	}
}

// GetDemoStatus returns the current state of demo data
func (h *Handler) GetDemoStatus(w http.ResponseWriter, r *http.Request) (int, any) {
	webhookStats, _ := h.webhookStorage.GetEventStats()

	return http.StatusOK, map[string]interface{}{
		"webhook_stats": webhookStats,
		"message":       "Demo environment status",
	}
}

// GenerateError returns intentional HTTP errors for testing
// Query params:
//   - code: HTTP status code (400-599, default: 500)
//   - message: Custom error message
//   - type: Error type (server, timeout, database, upstream, random)
func (h *Handler) GenerateError(w http.ResponseWriter, r *http.Request) (int, any) {
	code := 500
	if c := r.URL.Query().Get("code"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed >= 400 && parsed < 600 {
			code = parsed
		}
	}

	errorType := r.URL.Query().Get("type")
	if errorType == "" {
		errorType = "server"
	}

	// Handle random error type
	if errorType == "random" {
		errorTypes := []string{"server", "timeout", "database", "upstream", "validation"}
		errorType = errorTypes[code%len(errorTypes)]
		codes := []int{400, 401, 403, 404, 500, 502, 503, 504}
		code = codes[code%len(codes)]
	}

	customMessage := r.URL.Query().Get("message")

	var errorMsg string
	switch errorType {
	case "timeout":
		code = 504
		errorMsg = "Gateway timeout: upstream service did not respond in time"
	case "database":
		code = 500
		errorMsg = "Database connection failed: connection refused to postgres:5432"
	case "upstream":
		code = 502
		errorMsg = "Bad gateway: upstream service returned invalid response"
	case "validation":
		code = 400
		errorMsg = "Validation failed: required field 'id' is missing"
	case "auth":
		code = 401
		errorMsg = "Authentication required: invalid or expired token"
	case "forbidden":
		code = 403
		errorMsg = "Access denied: insufficient permissions for this resource"
	case "notfound":
		code = 404
		errorMsg = "Resource not found: the requested item does not exist"
	case "ratelimit":
		code = 429
		errorMsg = "Rate limit exceeded: too many requests, please retry later"
	case "unavailable":
		code = 503
		errorMsg = "Service unavailable: server is currently overloaded"
	default: // "server"
		errorMsg = "Internal server error: an unexpected error occurred"
	}

	if customMessage != "" {
		errorMsg = customMessage
	}

	return code, map[string]interface{}{
		"error":      errorMsg,
		"error_type": errorType,
		"status":     code,
		"demo":       true,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
