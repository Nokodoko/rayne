package webhooks

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
)

// Handler handles webhook HTTP requests
type Handler struct {
	storage   *Storage
	processor *Processor
}

// NewHandler creates a new webhook handler
func NewHandler(storage *Storage, processor *Processor) *Handler {
	return &Handler{
		storage:   storage,
		processor: processor,
	}
}

// sendDesktopNotification sends a notification to the local notify-server
// The server URL is configured via NOTIFY_SERVER_URL env var (default: http://host.minikube.internal:9999)
func sendDesktopNotification(payload WebhookPayload) {
	serverURL := os.Getenv("NOTIFY_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://host.minikube.internal:9999"
	}

	// Forward the full custom payload fields to the notify server
	notifyPayload := map[string]string{
		"ALERT_STATE":         payload.AlertState,
		"ALERT_TITLE":         payload.AlertTitleCustom,
		"APPLICATION_LONGNAME": payload.ApplicationLongname,
		"APPLICATION_TEAM":    payload.ApplicationTeam,
		"DETAILED_DESCRIPTION": payload.DetailedDescription,
		"IMPACT":              payload.Impact,
		"METRIC":              payload.Metric,
		"SUPPORT_GROUP":       payload.SupportGroup,
		"THRESHOLD":           payload.Threshold,
		"VALUE":               payload.Value,
		"URGENCY":             payload.Urgency,
	}

	jsonData, err := json.Marshal(notifyPayload)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to marshal notification payload: %v", err)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[WEBHOOK] Failed to send notification to %s: %v", serverURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Printf("[WEBHOOK] Desktop notification sent: %s", payload.AlertTitleCustom)
	} else {
		log.Printf("[WEBHOOK] Notification server returned status %d", resp.StatusCode)
	}
}

// ReceiveWebhook handles incoming webhooks from Datadog
func (h *Handler) ReceiveWebhook(w http.ResponseWriter, r *http.Request) (int, any) {
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid payload: " + err.Error()}
	}

	// Log the received payload
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("[WEBHOOK] Received payload:\n%s", string(payloadJSON))

	// Store the event
	event, err := h.storage.StoreEvent(payload)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to store event: " + err.Error()}
	}

	log.Printf("[WEBHOOK] Stored event ID: %d, Monitor: %s (%d), Status: %s",
		event.ID, payload.MonitorName, payload.MonitorID, payload.AlertStatus)

	// Send desktop notification with full payload
	go sendDesktopNotification(payload)

	// Process asynchronously
	go h.processor.Process(event)

	return http.StatusAccepted, map[string]interface{}{
		"event_id": event.ID,
		"status":   "accepted",
		"message":  "Webhook received and queued for processing",
	}
}

// GetWebhookEvents retrieves stored webhook events
func (h *Handler) GetWebhookEvents(w http.ResponseWriter, r *http.Request) (int, any) {
	// Parse pagination parameters
	page := 1
	perPage := 50

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}

	offset := (page - 1) * perPage

	events, totalCount, err := h.storage.GetRecentEvents(perPage, offset)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, WebhookEventListResponse{
		Events:     events,
		TotalCount: totalCount,
		Page:       page,
		PerPage:    perPage,
	}
}

// GetWebhookEvent retrieves a single webhook event by ID
func (h *Handler) GetWebhookEvent(w http.ResponseWriter, r *http.Request, idStr string) (int, any) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid event ID"}
	}

	event, err := h.storage.GetEventByID(id)
	if err != nil {
		return http.StatusNotFound, map[string]string{"error": "event not found"}
	}

	return http.StatusOK, event
}

// GetEventsByMonitor retrieves events for a specific monitor
func (h *Handler) GetEventsByMonitor(w http.ResponseWriter, r *http.Request, monitorIDStr string) (int, any) {
	monitorID, err := strconv.ParseInt(monitorIDStr, 10, 64)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid monitor ID"}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	events, err := h.storage.GetEventsByMonitorID(monitorID, limit)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, events
}

// CreateWebhook creates a new webhook in Datadog
func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) (int, any) {
	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body"}
	}

	if req.Name == "" || req.URL == "" {
		return http.StatusBadRequest, map[string]string{"error": "name and url are required"}
	}

	// Create webhook in Datadog
	result, status, err := requests.Post[WebhookResponse](w, r, urls.CreateWebhook, req)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return status, result
}

// SaveWebhookConfig saves a webhook configuration locally
func (h *Handler) SaveWebhookConfig(w http.ResponseWriter, r *http.Request) (int, any) {
	var config WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid config"}
	}

	if config.Name == "" || config.URL == "" {
		return http.StatusBadRequest, map[string]string{"error": "name and url are required"}
	}

	savedConfig, err := h.storage.SaveConfig(config)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusCreated, savedConfig
}

// GetWebhookConfigs retrieves all webhook configurations
func (h *Handler) GetWebhookConfigs(w http.ResponseWriter, r *http.Request) (int, any) {
	configs, err := h.storage.GetActiveConfigs()
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, configs
}

// GetWebhookStats retrieves webhook statistics
func (h *Handler) GetWebhookStats(w http.ResponseWriter, r *http.Request) (int, any) {
	stats, err := h.storage.GetEventStats()
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, stats
}

// ReprocessPending reprocesses all pending webhook events
func (h *Handler) ReprocessPending(w http.ResponseWriter, r *http.Request) (int, any) {
	if err := h.processor.ProcessPending(); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, map[string]string{"status": "reprocessing started"}
}

// ListProcessors returns the list of registered webhook processors
func (h *Handler) ListProcessors(w http.ResponseWriter, r *http.Request) (int, any) {
	processors := h.processor.ListProcessors()
	return http.StatusOK, map[string]interface{}{
		"processors": processors,
		"count":      len(processors),
	}
}
