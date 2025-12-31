package webhooks

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// ReceiveWebhook handles incoming webhooks from Datadog
func (h *Handler) ReceiveWebhook(w http.ResponseWriter, r *http.Request) (int, any) {
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid payload: " + err.Error()}
	}

	// Store the event
	event, err := h.storage.StoreEvent(payload)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to store event: " + err.Error()}
	}

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
