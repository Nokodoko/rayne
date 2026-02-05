package accounts

import (
	"errors"
	"time"
)

// Sentinel errors for account operations
var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrInvalidCredentials = errors.New("invalid datadog credentials")
	ErrDuplicateAccount   = errors.New("account already exists")
)

// Account represents a Datadog account configuration
type Account struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`       // e.g., "production-us"
	OrgID     int64     `json:"org_id"`     // Datadog org_id for webhook matching
	OrgName   string    `json:"org_name"`   // Display name
	APIKey    string    `json:"-"`          // Never exposed in JSON
	AppKey    string    `json:"-"`          // Never exposed in JSON
	BaseURL   string    `json:"base_url"`   // Default: https://api.datadoghq.com
	IsDefault bool      `json:"is_default"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Credentials holds the authentication info for Datadog API calls
type Credentials struct {
	APIKey  string
	AppKey  string
	BaseURL string // e.g., "https://api.datadoghq.com" (commercial) or "https://api.ddog-gov.com" (gov)
}

// ToCredentials converts an Account to Credentials
func (a *Account) ToCredentials() Credentials {
	return Credentials{
		APIKey:  a.APIKey,
		AppKey:  a.AppKey,
		BaseURL: a.BaseURL,
	}
}

// BuildURL constructs a full API URL from the credentials' base URL
func (c Credentials) BuildURL(path string) string {
	return c.BaseURL + path
}

// Common Datadog API paths
const (
	PathDowntime  = "/api/v2/downtime"
	PathEvents    = "/api/v1/events"
	PathHosts     = "/api/v1/hosts"
	PathMonitors  = "/api/v1/monitor"
	PathNotebooks = "/api/v1/notebooks"
)

// Common Datadog base URLs
const (
	BaseURLGov        = "https://api.ddog-gov.com"
	BaseURLCommercial = "https://api.datadoghq.com"
	BaseURLEU         = "https://api.datadoghq.eu"
	BaseURLUS3        = "https://api.us3.datadoghq.com"
	BaseURLUS5        = "https://api.us5.datadoghq.com"
	BaseURLAP1        = "https://api.ap1.datadoghq.com"
)

// CreateAccountRequest represents a request to create a new account
type CreateAccountRequest struct {
	Name    string `json:"name"`
	OrgID   int64  `json:"org_id,omitempty"`
	OrgName string `json:"org_name,omitempty"`
	APIKey  string `json:"api_key"`
	AppKey  string `json:"app_key"`
	BaseURL string `json:"base_url,omitempty"` // Defaults to BaseURLCommercial
}

// UpdateAccountRequest represents a request to update an account
type UpdateAccountRequest struct {
	OrgID   *int64  `json:"org_id,omitempty"`
	OrgName *string `json:"org_name,omitempty"`
	APIKey  *string `json:"api_key,omitempty"`
	AppKey  *string `json:"app_key,omitempty"`
	BaseURL *string `json:"base_url,omitempty"`
	Active  *bool   `json:"active,omitempty"`
}

// AccountResponse represents an account for API responses (no sensitive data)
type AccountResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	OrgID     int64     `json:"org_id"`
	OrgName   string    `json:"org_name"`
	BaseURL   string    `json:"base_url"`
	IsDefault bool      `json:"is_default"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToResponse converts an Account to AccountResponse (safe for API output)
func (a *Account) ToResponse() AccountResponse {
	return AccountResponse{
		ID:        a.ID,
		Name:      a.Name,
		OrgID:     a.OrgID,
		OrgName:   a.OrgName,
		BaseURL:   a.BaseURL,
		IsDefault: a.IsDefault,
		Active:    a.Active,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

// TestConnectionResult represents the result of testing account credentials
type TestConnectionResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
	BaseURL string `json:"base_url"`
	OrgID   int64  `json:"org_id,omitempty"`
	OrgName string `json:"org_name,omitempty"`
}
