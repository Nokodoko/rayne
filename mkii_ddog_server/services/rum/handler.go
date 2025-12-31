package rum

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Handler handles RUM HTTP requests
type Handler struct {
	storage *Storage
}

// NewHandler creates a new RUM handler
func NewHandler(storage *Storage) *Handler {
	return &Handler{storage: storage}
}

// InitVisitor initializes a visitor session
// Called when a page loads to get or create a visitor UUID
func (h *Handler) InitVisitor(w http.ResponseWriter, r *http.Request) (int, any) {
	var req VisitorInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, use headers
		req.ExistingUUID = r.Header.Get("X-Visitor-UUID")
		req.UserAgent = r.UserAgent()
		req.Referrer = r.Header.Get("Referer")
	}

	if req.UserAgent == "" {
		req.UserAgent = r.UserAgent()
	}

	// Check for existing visitor
	if req.ExistingUUID != "" {
		visitor, err := h.storage.GetVisitorByUUID(req.ExistingUUID)
		if err == nil && visitor != nil {
			// Existing visitor - create new session
			sessionID := uuid.New().String()

			if err := h.storage.UpdateVisitorLastSeen(req.ExistingUUID); err != nil {
				return http.StatusInternalServerError, map[string]string{"error": "failed to update visitor"}
			}

			if err := h.storage.CreateSession(req.ExistingUUID, sessionID, req.Referrer, req.EntryPage, req.UserAgent); err != nil {
				return http.StatusInternalServerError, map[string]string{"error": "failed to create session"}
			}

			return http.StatusOK, VisitorInitResponse{
				VisitorUUID: req.ExistingUUID,
				SessionID:   sessionID,
				IsNew:       false,
				Message:     "Welcome back!",
			}
		}
	}

	// Create new visitor
	visitorUUID := uuid.New().String()
	sessionID := uuid.New().String()

	// Hash IP for privacy
	ipHash := hashIP(getClientIP(r))

	if err := h.storage.CreateVisitor(visitorUUID, req.UserAgent, ipHash); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to create visitor"}
	}

	if err := h.storage.CreateSession(visitorUUID, sessionID, req.Referrer, req.EntryPage, req.UserAgent); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to create session"}
	}

	return http.StatusCreated, VisitorInitResponse{
		VisitorUUID: visitorUUID,
		SessionID:   sessionID,
		IsNew:       true,
		Message:     "Welcome, new visitor!",
	}
}

// TrackEvent records a RUM event
func (h *Handler) TrackEvent(w http.ResponseWriter, r *http.Request) (int, any) {
	var req TrackEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body"}
	}

	if req.VisitorUUID == "" || req.SessionID == "" || req.EventType == "" {
		return http.StatusBadRequest, map[string]string{"error": "visitor_uuid, session_id, and event_type are required"}
	}

	// Validate event type
	validTypes := map[string]bool{
		"view":      true,
		"action":    true,
		"error":     true,
		"resource":  true,
		"long_task": true,
	}
	if !validTypes[req.EventType] {
		return http.StatusBadRequest, map[string]string{"error": "invalid event_type"}
	}

	event := RUMEvent{
		VisitorUUID: req.VisitorUUID,
		SessionID:   req.SessionID,
		EventType:   req.EventType,
		Timestamp:   time.Now(),
		PageURL:     req.PageURL,
		PageTitle:   req.PageTitle,
		ActionName:  req.ActionName,
		ActionType:  req.ActionType,
		ErrorMsg:    req.ErrorMsg,
		Duration:    req.Duration,
		Metadata:    req.Metadata,
	}

	if err := h.storage.StoreEvent(event); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to store event"}
	}

	return http.StatusAccepted, map[string]string{"status": "event recorded"}
}

// EndSession ends a visitor session
func (h *Handler) EndSession(w http.ResponseWriter, r *http.Request) (int, any) {
	var req SessionEndRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body"}
	}

	if req.SessionID == "" {
		return http.StatusBadRequest, map[string]string{"error": "session_id is required"}
	}

	if err := h.storage.EndSession(req.SessionID, req.ExitPage); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to end session"}
	}

	return http.StatusOK, map[string]string{"status": "session ended"}
}

// GetVisitor retrieves visitor information
func (h *Handler) GetVisitor(w http.ResponseWriter, r *http.Request, visitorUUID string) (int, any) {
	visitor, err := h.storage.GetVisitorByUUID(visitorUUID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}
	if visitor == nil {
		return http.StatusNotFound, map[string]string{"error": "visitor not found"}
	}

	return http.StatusOK, visitor
}

// GetSession retrieves session information
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request, sessionID string) (int, any) {
	session, err := h.storage.GetSessionByID(sessionID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}
	if session == nil {
		return http.StatusNotFound, map[string]string{"error": "session not found"}
	}

	// Get events for the session
	events, _ := h.storage.GetEventsBySession(sessionID)

	return http.StatusOK, map[string]interface{}{
		"session": session,
		"events":  events,
	}
}

// GetUniqueVisitors returns count of unique visitors
func (h *Handler) GetUniqueVisitors(w http.ResponseWriter, r *http.Request) (int, any) {
	from, to := parseTimeRange(r)

	count, err := h.storage.CountUniqueVisitors(from, to)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, map[string]interface{}{
		"unique_visitors": count,
		"from":            from.Format(time.RFC3339),
		"to":              to.Format(time.RFC3339),
	}
}

// GetAnalytics returns comprehensive analytics
func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) (int, any) {
	from, to := parseTimeRange(r)

	analytics, err := h.storage.GetAnalytics(from, to)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, analytics
}

// GetRecentSessions returns recent sessions
func (h *Handler) GetRecentSessions(w http.ResponseWriter, r *http.Request) (int, any) {
	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	sessions, err := h.storage.GetRecentSessions(limit, offset)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": err.Error()}
	}

	return http.StatusOK, sessions
}

// Helper functions

func hashIP(ip string) string {
	hash := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(hash[:])
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now()
	from := now.Add(-24 * time.Hour) // Default: last 24 hours
	to := now

	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := time.Parse(time.RFC3339, f); err == nil {
			from = parsed
		} else if parsed, err := time.Parse("2006-01-02", f); err == nil {
			from = parsed
		}
	}

	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		} else if parsed, err := time.Parse("2006-01-02", t); err == nil {
			to = parsed.Add(24*time.Hour - time.Second) // End of day
		}
	}

	// Handle relative time ranges
	if period := r.URL.Query().Get("period"); period != "" {
		switch period {
		case "1h":
			from = now.Add(-1 * time.Hour)
		case "6h":
			from = now.Add(-6 * time.Hour)
		case "12h":
			from = now.Add(-12 * time.Hour)
		case "24h", "1d":
			from = now.Add(-24 * time.Hour)
		case "7d":
			from = now.Add(-7 * 24 * time.Hour)
		case "30d":
			from = now.Add(-30 * 24 * time.Hour)
		}
	}

	return from, to
}
