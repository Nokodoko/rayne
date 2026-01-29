package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/requests"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/urls"
	"github.com/Nokodoko/mkii_ddog_server/services/accounts"
)

// AccountResolver interface defined at consumer side (Go best practice)
// This allows webhook handler to be tested with mock implementations
type AccountResolver interface {
	ResolveAccount(orgID int64, accountName string) *accounts.Account
	GetDefault() *accounts.Account
}

// Handler handles webhook HTTP requests
type Handler struct {
	storage    *Storage
	dispatcher *Dispatcher
	processor  *Processor        // Legacy processor for backwards compatibility
	accounts   AccountResolver   // Optional: for multi-account support
}

// NewHandler creates a new webhook handler with dispatcher
func NewHandler(storage *Storage, dispatcher *Dispatcher) *Handler {
	return &Handler{
		storage:    storage,
		dispatcher: dispatcher,
	}
}

// NewHandlerWithAccounts creates a new webhook handler with multi-account support
func NewHandlerWithAccounts(storage *Storage, dispatcher *Dispatcher, accounts AccountResolver) *Handler {
	return &Handler{
		storage:    storage,
		dispatcher: dispatcher,
		accounts:   accounts,
	}
}

// NewHandlerLegacy creates a handler with the legacy processor (for backwards compatibility)
func NewHandlerLegacy(storage *Storage, processor *Processor) *Handler {
	return &Handler{
		storage:   storage,
		processor: processor,
	}
}

// ReceiveWebhook handles incoming webhooks from Datadog
func (h *Handler) ReceiveWebhook(w http.ResponseWriter, r *http.Request) (int, any) {
	return h.receiveWebhookInternal(w, r, "")
}

// ReceiveWebhookForAccount handles incoming webhooks with explicit account routing
func (h *Handler) ReceiveWebhookForAccount(w http.ResponseWriter, r *http.Request, accountName string) (int, any) {
	return h.receiveWebhookInternal(w, r, accountName)
}

// receiveWebhookInternal is the internal implementation for webhook receiving
func (h *Handler) receiveWebhookInternal(w http.ResponseWriter, r *http.Request, explicitAccountName string) (int, any) {
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid payload: " + err.Error()}
	}

	// Log the received payload
	payloadJSON, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("[WEBHOOK] Received payload:\n%s", string(payloadJSON))

	// Resolve account from OrgID in payload or explicit account name
	var accountID *int64
	var accountName string

	if h.accounts != nil {
		account := h.accounts.ResolveAccount(payload.OrgID, explicitAccountName)
		if account != nil {
			accountID = &account.ID
			accountName = account.Name
			log.Printf("[WEBHOOK] Resolved account: %s (ID: %d) for org_id: %d",
				accountName, account.ID, payload.OrgID)
		} else if explicitAccountName != "" {
			// Explicit account requested but not found
			return http.StatusNotFound, map[string]string{
				"error": fmt.Sprintf("account not found: %s", explicitAccountName),
			}
		}
	}

	// Store the event with account association
	event, err := h.storage.StoreEventWithAccount(payload, accountID, accountName)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to store event: " + err.Error()}
	}

	log.Printf("[WEBHOOK] Stored event ID: %d, Monitor: %s (%d), Status: %s, Account: %s",
		event.ID, payload.MonitorName, payload.MonitorID, payload.AlertStatus, accountName)

	// Submit to dispatcher for bounded, coordinated processing
	// This replaces the fire-and-forget goroutines with proper backpressure
	// IMPORTANT: Use context.Background() instead of r.Context() because the HTTP
	// request context is cancelled after the handler returns, but processing is async
	if h.dispatcher != nil {
		if err := h.dispatcher.Submit(context.Background(), event); err != nil {
			log.Printf("[WEBHOOK] Warning: dispatcher queue full, event %d stored but processing delayed: %v",
				event.ID, err)
			// Event is still stored, will be processed when queue has capacity
			// or via manual reprocessing
		}
	} else if h.processor != nil {
		// Legacy fallback: fire-and-forget (backwards compatibility)
		go h.processor.Process(event)
	}

	response := map[string]any{
		"event_id": event.ID,
		"status":   "accepted",
		"message":  "Webhook received and queued for processing",
	}
	if accountName != "" {
		response["account_name"] = accountName
	}

	return http.StatusAccepted, response
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
	// Get pending events
	events, _, err := h.storage.GetRecentEvents(100, 0)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	count := 0
	for _, event := range events {
		if event.Status == "pending" {
			eventCopy := event
			if h.dispatcher != nil {
				if err := h.dispatcher.Submit(r.Context(), &eventCopy); err == nil {
					count++
				}
			} else if h.processor != nil {
				go h.processor.Process(&eventCopy)
				count++
			}
		}
	}

	return http.StatusOK, map[string]interface{}{
		"status":  "reprocessing started",
		"queued":  count,
	}
}

// ListProcessors returns the list of registered webhook processors
func (h *Handler) ListProcessors(w http.ResponseWriter, r *http.Request) (int, any) {
	var processors []string
	if h.processor != nil {
		processors = h.processor.ListProcessors()
	}

	return http.StatusOK, map[string]interface{}{
		"processors": processors,
		"count":      len(processors),
	}
}

// GetDispatcherStats returns dispatcher statistics
func (h *Handler) GetDispatcherStats(w http.ResponseWriter, r *http.Request) (int, any) {
	if h.dispatcher == nil {
		return http.StatusOK, map[string]string{"status": "dispatcher not configured"}
	}

	stats := h.dispatcher.Stats()
	return http.StatusOK, stats
}

// TestNotify sends a test notification directly to configured servers (bypasses orchestrator)
func (h *Handler) TestNotify(w http.ResponseWriter, r *http.Request) (int, any) {
	urlsEnv := os.Getenv("NOTIFY_SERVER_URLS")
	if urlsEnv == "" {
		urlsEnv = os.Getenv("NOTIFY_SERVER_URL")
	}
	if urlsEnv == "" {
		urlsEnv = "http://host.minikube.internal:9999"
	}

	results := make(map[string]string)
	payload := []byte(`{"title":"Test","message":"Direct test notification","urgency":"normal"}`)

	for _, serverURL := range strings.Split(urlsEnv, ",") {
		serverURL = strings.TrimSpace(serverURL)
		if serverURL == "" {
			continue
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(serverURL, "application/json", bytes.NewBuffer(payload))
		if err != nil {
			results[serverURL] = "error: " + err.Error()
			continue
		}
		resp.Body.Close()
		results[serverURL] = "status: " + resp.Status
	}

	return http.StatusOK, map[string]interface{}{
		"urls":    urlsEnv,
		"results": results,
	}
}
