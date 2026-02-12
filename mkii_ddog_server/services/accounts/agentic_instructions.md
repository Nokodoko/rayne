# agentic_instructions.md

## Purpose
Multi-account Datadog credential management. Defines the Account model and Credentials type for supporting multiple Datadog organizations with different API endpoints (Commercial, Gov, EU, US3, US5, AP1).

## Technology
Go, errors

## Contents
- `types.go` -- Account, Credentials, request/response types, base URL constants, API path constants

## Key Functions
- `(a *Account) ToCredentials() Credentials` -- Converts Account to API-callable credentials
- `(c Credentials) BuildURL(path string) string` -- Constructs full API URL from base + path
- `(a *Account) ToResponse() AccountResponse` -- Strips sensitive fields (APIKey, AppKey) for API output

## Data Types
- `Account` -- struct: ID, Name, OrgID, OrgName, APIKey (json:"-"), AppKey (json:"-"), BaseURL, IsDefault, Active, CreatedAt, UpdatedAt
- `Credentials` -- struct: APIKey, AppKey, BaseURL
- `CreateAccountRequest` -- struct: Name, OrgID, OrgName, APIKey, AppKey, BaseURL
- `UpdateAccountRequest` -- struct: all pointer fields for partial updates
- `AccountResponse` -- struct: safe for API output (no keys)
- `TestConnectionResult` -- struct: Valid, Message, BaseURL, OrgID, OrgName
- Constants: `BaseURLGov`, `BaseURLCommercial`, `BaseURLEU`, `BaseURLUS3`, `BaseURLUS5`, `BaseURLAP1`
- Path constants: `PathDowntime`, `PathEvents`, `PathHosts`, `PathMonitors`, `PathNotebooks`
- Sentinel errors: `ErrAccountNotFound`, `ErrInvalidCredentials`, `ErrDuplicateAccount`

## Logging
None

## CRUD Entry Points
- **Create**: Instantiate `CreateAccountRequest`, persist to storage (storage not yet implemented in this package)
- **Read**: Use `Account.ToCredentials()` and `Credentials.BuildURL()` for API calls
- **Update**: Use `UpdateAccountRequest` with pointer fields for partial updates
- **Delete**: N/A (types only, no storage in this package)

## Style Guide
- Sensitive fields tagged `json:"-"` to prevent accidental exposure
- Pointer fields in update requests for partial updates (nil = no change)
- Representative snippet:

```go
const (
	BaseURLGov        = "https://api.ddog-gov.com"
	BaseURLCommercial = "https://api.datadoghq.com"
)

func (a *Account) ToCredentials() Credentials {
	return Credentials{
		APIKey:  a.APIKey,
		AppKey:  a.AppKey,
		BaseURL: a.BaseURL,
	}
}

func (c Credentials) BuildURL(path string) string {
	return c.BaseURL + path
}
```
