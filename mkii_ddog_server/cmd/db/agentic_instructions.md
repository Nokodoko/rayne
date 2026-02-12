# agentic_instructions.md

## Purpose
PostgreSQL database connection factory with Datadog APM tracing on all SQL queries.

## Technology
Go, database/sql, github.com/lib/pq, dd-trace-go/contrib/database/sql (sqltrace)

## Contents
- `db.go` -- SqlStorage() connection factory

## Key Functions
- `SqlStorage(cfg config.Config) (*sql.DB, error)` -- Registers pq driver with Datadog tracing, opens connection, pings, configures connection pool (25 max open, 5 idle, 5min lifetime, 1min idle timeout)

## Data Types
None (returns stdlib `*sql.DB`)

## Logging
Uses `log.Printf` for connection status and `log.Println` for success.

## CRUD Entry Points
- **Create**: Instantiate via `db.SqlStorage(config.Envs)` from main.go
- **Read**: N/A -- returns `*sql.DB` handle
- **Update**: Modify pool settings (MaxOpenConns, MaxIdleConns, etc.)
- **Delete**: N/A

## Style Guide
- Connection string built via `fmt.Sprintf` with named parameters
- Error wrapping with `fmt.Errorf("...: %w", err)`
- Representative snippet:

```go
func SqlStorage(cfg config.Config) (*sql.DB, error) {
	sqltrace.Register("postgres", &pq.Driver{}, sqltrace.WithServiceName("rayne-db"))
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.SSLMode,
	)
	db, err := sqltrace.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}
```
