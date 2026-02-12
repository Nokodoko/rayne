# agentic_instructions.md

## Purpose
Datadog API key retrieval from environment variables and credential management for multi-account support.

## Technology
Go, os.LookupEnv

## Contents
- `keys.go` -- Api(), App() functions, Credentials struct, Default() constructor, BuildURL() method

## Key Functions
- `Api() string` -- Returns DD_API_KEY from env or error message
- `App() string` -- Returns DD_APP_KEY from env or error message
- `Default() Credentials` -- Returns credentials from env vars with DefaultBaseURL
- `(c Credentials) BuildURL(path string) string` -- Constructs full API URL

## Data Types
- `Credentials` -- struct: APIKey, AppKey, BaseURL (all string)

## Logging
None

## CRUD Entry Points
- **Create**: N/A
- **Read**: Call `keys.Api()`, `keys.App()`, or `keys.Default()`
- **Update**: Modify DefaultBaseURL constant or add new credential sources
- **Delete**: N/A

## Style Guide
- Package-level constants for env var names and default URL
- Switch-case pattern for env lookup
- Representative snippet:

```go
const DefaultBaseURL = "https://api.datadoghq.com"

func Api() string {
	_, ok := os.LookupEnv(apiKeyName)
	switch {
	case !ok:
		return fmt.Sprintf("%s environment variable not set", apiKeyName)
	default:
		return os.Getenv(apiKeyName)
	}
}
```
