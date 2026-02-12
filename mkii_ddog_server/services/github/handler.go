package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const maxBodySize = 1 << 20 // 1 MB

// Handler handles GitHub webhook HTTP requests.
type Handler struct {
	storage     *Storage
	secret      string
	notifier    *Notifier
	agentClient *AgentClient
}

// NewHandler creates a new GitHub webhook handler.
func NewHandler(storage *Storage) *Handler {
	return &Handler{
		storage:     storage,
		secret:      os.Getenv("GITHUB_WEBHOOK_SECRET"),
		notifier:    NewNotifier(),
		agentClient: NewAgentClient(),
	}
}

// ReceiveIssueEvent handles incoming GitHub issue webhook events.
// Verifies HMAC-SHA256 signature, filters for "issues" event type, deduplicates
// by X-GitHub-Delivery header, and stores the payload.
func (h *Handler) ReceiveIssueEvent(w http.ResponseWriter, r *http.Request) (int, any) {
	// Verify X-GitHub-Event header
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "ping" {
		return http.StatusOK, map[string]string{"status": "pong"}
	}
	if eventType != "issues" {
		return http.StatusBadRequest, map[string]string{"error": "unsupported event type"}
	}

	// Require webhook secret in production
	if h.secret == "" {
		log.Println("[GITHUB] CRITICAL: GITHUB_WEBHOOK_SECRET is not set, rejecting request")
		return http.StatusInternalServerError, map[string]string{"error": "webhook secret not configured"}
	}

	// Require delivery ID for deduplication
	deliveryID := r.Header.Get("X-GitHub-Delivery")
	if deliveryID == "" {
		return http.StatusBadRequest, map[string]string{"error": "missing delivery ID"}
	}

	// Limit body size to prevent OOM
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusRequestEntityTooLarge, map[string]string{"error": "payload too large"}
	}

	// Verify HMAC-SHA256 signature
	sig := r.Header.Get("X-Hub-Signature-256")
	if !verifySignature(h.secret, sig, body) {
		return http.StatusUnauthorized, map[string]string{"error": "invalid signature"}
	}

	var payload IssueEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[GITHUB] Invalid payload: %v", err)
		return http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"}
	}

	log.Printf("[GITHUB] Received issue event: action=%s repo=%s issue=#%d %q delivery=%s",
		payload.Action, payload.Repo.FullName, payload.Issue.Number, payload.Issue.Title, deliveryID)

	stored, err := h.storage.StoreEvent(payload, deliveryID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return http.StatusOK, map[string]string{"status": "already processed", "delivery_id": deliveryID}
		}
		log.Printf("[GITHUB] Failed to store event: %v", err)
		return http.StatusInternalServerError, map[string]string{"error": "internal server error"}
	}

	// Fire-and-forget desktop notification
	go h.notifier.NotifyIssueEvent(payload)

	// Process opened issues with Claude agent
	if payload.Action == "opened" {
		processed, branchName, _ := h.storage.HasBeenProcessed(payload.Issue.ID, payload.Repo.FullName)
		if processed {
			log.Printf("[GITHUB] Issue #%d already processed, branch: %s", payload.Issue.Number, branchName)
		} else {
			go h.processIssueWithAgent(stored, payload)
		}
	}

	return http.StatusAccepted, map[string]any{
		"event_id":    stored.ID,
		"status":      "accepted",
		"action":      stored.Action,
		"issue":       stored.Number,
		"repo":        stored.RepoName,
		"delivery_id": deliveryID,
	}
}

// GetIssueEvents retrieves stored GitHub issue events with pagination.
func (h *Handler) GetIssueEvents(w http.ResponseWriter, r *http.Request) (int, any) {
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
		log.Printf("[GITHUB] Failed to get events: %v", err)
		return http.StatusInternalServerError, map[string]string{"error": "internal server error"}
	}

	return http.StatusOK, IssueEventListResponse{
		Events:     events,
		TotalCount: totalCount,
		Page:       page,
		PerPage:    perPage,
	}
}

// GetIssueEvent retrieves a single GitHub issue event by ID.
func (h *Handler) GetIssueEvent(w http.ResponseWriter, r *http.Request, idStr string) (int, any) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid event ID"}
	}

	event, err := h.storage.GetEventByID(id)
	if err == sql.ErrNoRows {
		return http.StatusNotFound, map[string]string{"error": "event not found"}
	}
	if err != nil {
		log.Printf("[GITHUB] Failed to get event %d: %v", id, err)
		return http.StatusInternalServerError, map[string]string{"error": "internal server error"}
	}

	return http.StatusOK, event
}

// GetIssueStats returns statistics about stored GitHub issue events.
func (h *Handler) GetIssueStats(w http.ResponseWriter, r *http.Request) (int, any) {
	stats, err := h.storage.GetStats()
	if err != nil {
		log.Printf("[GITHUB] Failed to get stats: %v", err)
		return http.StatusInternalServerError, map[string]string{"error": "internal server error"}
	}

	return http.StatusOK, stats
}

// processIssueWithAgent invokes the Claude agent sidecar to implement the feature request.
func (h *Handler) processIssueWithAgent(stored *StoredIssueEvent, payload IssueEvent) {
	log.Printf("[GITHUB] Starting agent processing for issue #%d: %s", payload.Issue.Number, payload.Issue.Title)

	if err := h.storage.UpdateAgentStatus(stored.ID, string(AgentStatusProcessing), "", ""); err != nil {
		log.Printf("[GITHUB] Failed to update agent status: %v", err)
	}

	// Fetch past processed issues for cross-reference
	pastIssues, err := h.storage.GetCompletedIssues(payload.Repo.FullName)
	if err != nil {
		log.Printf("[GITHUB] Warning: failed to fetch past issues: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	req := AgentProcessRequest{
		IssueNumber: payload.Issue.Number,
		IssueTitle:  payload.Issue.Title,
		IssueBody:   payload.Issue.Body,
		IssueURL:    payload.Issue.HTMLURL,
		RepoName:    payload.Repo.FullName,
		SenderLogin: payload.Sender.Login,
		EventID:     stored.ID,
		PastIssues:  pastIssues,
	}

	resp, err := h.agentClient.ProcessIssue(ctx, req)
	if err != nil {
		log.Printf("[GITHUB] Agent processing failed for issue #%d: %v", payload.Issue.Number, err)
		errJSON := fmt.Sprintf(`{"error":%q}`, err.Error())
		h.storage.UpdateAgentStatus(stored.ID, string(AgentStatusFailed), errJSON, "")
		go h.notifier.NotifyAgentResult(payload.Issue.Number, payload.Issue.Title, payload.Repo.FullName, false)
		return
	}

	// Check if the agent detected this as a duplicate
	if resp.Duplicate {
		resultJSON, _ := json.Marshal(resp)
		h.storage.UpdateAgentStatus(stored.ID, string(AgentStatusSkipped), string(resultJSON), "")
		go h.notifier.NotifyAgentResult(payload.Issue.Number, payload.Issue.Title, payload.Repo.FullName, true)
		log.Printf("[GITHUB] Issue #%d detected as duplicate of #%d, skipped", payload.Issue.Number, resp.DuplicateOf)
		return
	}

	resultJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[GITHUB] Failed to marshal agent response for issue #%d: %v", payload.Issue.Number, err)
		resultJSON = []byte(`{"error":"failed to marshal response"}`)
	}
	h.storage.UpdateAgentStatus(stored.ID, string(AgentStatusCompleted), string(resultJSON), resp.BranchName)
	go h.notifier.NotifyAgentResult(payload.Issue.Number, payload.Issue.Title, payload.Repo.FullName, true)
	log.Printf("[GITHUB] Agent completed issue #%d, branch: %s", payload.Issue.Number, resp.BranchName)
}

// verifySignature validates the GitHub HMAC-SHA256 webhook signature.
func verifySignature(secret, signature string, body []byte) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}
