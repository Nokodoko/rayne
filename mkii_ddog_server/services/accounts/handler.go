package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

// Handler handles account HTTP requests
type Handler struct {
	manager *AccountManager
	client  *http.Client
}

// NewHandler creates a new account handler
func NewHandler(manager *AccountManager) *Handler {
	return &Handler{
		manager: manager,
		client: httptrace.WrapClient(&http.Client{
			Timeout: 10 * time.Second,
		}),
	}
}

// ListAccounts retrieves all accounts (GET /v1/accounts)
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) (int, any) {
	accounts, err := h.manager.GetAll()
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("list accounts: %v", err)}
	}

	// Convert to safe response format (no credentials)
	responses := make([]AccountResponse, len(accounts))
	for i, acct := range accounts {
		responses[i] = acct.ToResponse()
	}

	return http.StatusOK, map[string]any{
		"accounts": responses,
		"count":    len(responses),
	}
}

// CreateAccount creates a new account (POST /v1/accounts)
func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) (int, any) {
	var req CreateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid request: %v", err)}
	}

	// Validation
	if req.Name == "" {
		return http.StatusBadRequest, map[string]string{"error": "name is required"}
	}
	if req.APIKey == "" {
		return http.StatusBadRequest, map[string]string{"error": "api_key is required"}
	}
	if req.AppKey == "" {
		return http.StatusBadRequest, map[string]string{"error": "app_key is required"}
	}

	// Default base URL
	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = BaseURLGov
	}

	account := Account{
		Name:    req.Name,
		OrgID:   req.OrgID,
		OrgName: req.OrgName,
		APIKey:  req.APIKey,
		AppKey:  req.AppKey,
		BaseURL: baseURL,
		Active:  true,
	}

	created, err := h.manager.Create(account)
	if err != nil {
		if errors.Is(err, ErrDuplicateAccount) {
			return http.StatusConflict, map[string]string{"error": "account with this name already exists"}
		}
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("create account: %v", err)}
	}

	return http.StatusCreated, created.ToResponse()
}

// GetAccount retrieves an account by name (GET /v1/accounts/{name})
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	account, err := h.manager.GetByName(name)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return http.StatusNotFound, map[string]string{"error": fmt.Sprintf("account not found: %s", name)}
		}
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("get account: %v", err)}
	}

	return http.StatusOK, account.ToResponse()
}

// UpdateAccount updates an account (PUT /v1/accounts/{name})
func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	// Get existing account
	existing, err := h.manager.GetByName(name)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return http.StatusNotFound, map[string]string{"error": fmt.Sprintf("account not found: %s", name)}
		}
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("get account: %v", err)}
	}

	var req UpdateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid request: %v", err)}
	}

	// Apply updates
	if req.OrgID != nil {
		existing.OrgID = *req.OrgID
	}
	if req.OrgName != nil {
		existing.OrgName = *req.OrgName
	}
	if req.APIKey != nil {
		existing.APIKey = *req.APIKey
	}
	if req.AppKey != nil {
		existing.AppKey = *req.AppKey
	}
	if req.BaseURL != nil {
		existing.BaseURL = *req.BaseURL
	}
	if req.Active != nil {
		existing.Active = *req.Active
	}

	updated, err := h.manager.Update(existing.ID, *existing)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("update account: %v", err)}
	}

	return http.StatusOK, updated.ToResponse()
}

// DeleteAccount deletes an account (DELETE /v1/accounts/{name})
func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	account, err := h.manager.GetByName(name)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return http.StatusNotFound, map[string]string{"error": fmt.Sprintf("account not found: %s", name)}
		}
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("get account: %v", err)}
	}

	if account.IsDefault {
		return http.StatusBadRequest, map[string]string{"error": "cannot delete default account"}
	}

	if err := h.manager.Delete(account.ID); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("delete account: %v", err)}
	}

	return http.StatusOK, map[string]string{"message": fmt.Sprintf("account %s deleted", name)}
}

// SetDefaultAccount sets an account as the default (POST /v1/accounts/{name}/default)
func (h *Handler) SetDefaultAccount(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	account, err := h.manager.GetByName(name)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return http.StatusNotFound, map[string]string{"error": fmt.Sprintf("account not found: %s", name)}
		}
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("get account: %v", err)}
	}

	if err := h.manager.SetDefault(account.ID); err != nil {
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("set default: %v", err)}
	}

	return http.StatusOK, map[string]string{"message": fmt.Sprintf("account %s set as default", name)}
}

// TestConnection tests account credentials against Datadog API (POST /v1/accounts/{name}/test)
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	account, err := h.manager.GetByName(name)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return http.StatusNotFound, map[string]string{"error": fmt.Sprintf("account not found: %s", name)}
		}
		return http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("get account: %v", err)}
	}

	result := h.testCredentials(account)
	return http.StatusOK, result
}

// testCredentials validates credentials against Datadog API
func (h *Handler) testCredentials(account *Account) TestConnectionResult {
	result := TestConnectionResult{
		BaseURL: account.BaseURL,
	}

	// Use the validate endpoint to test credentials
	url := account.BaseURL + "/api/v1/validate"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Valid = false
		result.Message = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	req.Header.Set("DD-API-KEY", account.APIKey)
	req.Header.Set("DD-APPLICATION-KEY", account.AppKey)
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		result.Valid = false
		result.Message = fmt.Sprintf("connection failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		result.Valid = true
		result.Message = "credentials validated successfully"

		// Try to parse response for org info
		var validateResp struct {
			Valid bool `json:"valid"`
		}
		body, _ := io.ReadAll(resp.Body)
		if json.Unmarshal(body, &validateResp) == nil && validateResp.Valid {
			result.Valid = true
		}
	} else if resp.StatusCode == http.StatusForbidden {
		result.Valid = false
		result.Message = "invalid API key"
	} else {
		result.Valid = false
		result.Message = fmt.Sprintf("validation failed with status: %d", resp.StatusCode)
	}

	return result
}

// GetStats returns account manager statistics (GET /v1/accounts/stats)
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) (int, any) {
	return http.StatusOK, h.manager.Stats()
}
