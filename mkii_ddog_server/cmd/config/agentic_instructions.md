# agentic_instructions.md

## Purpose
Environment variable configuration loading for the Go server. Centralizes all config with defaults.

## Technology
Go, os.LookupEnv

## Contents
- `env.go` -- Config struct definition and initConfig() initializer

## Key Functions
- `initConfig() Config` -- Reads environment variables with fallback defaults, returns populated Config struct

## Data Types
- `Config` -- struct with fields: PublicHost, DBHost, DBPort, DBUser, DBPassword, DBName, SSLMode, DDService, DDEnv, DDVersion, DDAgentHost, DDSite (all string)

## Logging
None. Pure config loading.

## CRUD Entry Points
- **Create**: Add a new field to `Config` struct and a corresponding `utils.GetEnv()` call in `initConfig()`
- **Read**: Access via `config.Envs.<FieldName>`
- **Update**: Change default values in `initConfig()`
- **Delete**: Remove field from struct and init function

## Style Guide
- PascalCase for exported struct fields
- Config loaded once via `var Envs = initConfig()` package-level variable
- Representative snippet:

```go
type Config struct {
	PublicHost string
	DBHost     string
	DBPort     string
	DDService  string
	DDEnv      string
}

var Envs = initConfig()

func initConfig() Config {
	return Config{
		PublicHost: utils.GetEnv("PUBLIC_HOST", "http://localhost"),
		DBHost:     utils.GetEnv("DB_HOST", "localhost"),
		DDService:  utils.GetEnv("DD_SERVICE", "rayne"),
	}
}
```
