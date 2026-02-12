# agentic_instructions.md

## Purpose
Application entry point (main package). Initializes APM tracer, database connection, and HTTP server with graceful shutdown.

## Technology
Go, Datadog APM tracer, POSIX signal handling (SIGTERM, SIGINT)

## Contents
- `main.go` -- main() function, signal handling, tracer lifecycle, server startup

## Key Functions
- `main()` -- Creates root context with cancellation, starts APM tracer, opens DB, launches DDogServer, waits for shutdown signal
- `initStorage(db *sql.DB)` -- Legacy ping-and-log function (unused but present)

## Data Types
None

## Logging
Uses `log.Printf` with descriptive messages for tracer start, signal receipt, shutdown.

## CRUD Entry Points
- **Create**: N/A -- this is the entry point
- **Read**: N/A
- **Update**: Modify server address (`:8080`), tracer options, or startup sequence
- **Delete**: N/A

## Style Guide
- Graceful shutdown via context cancellation + signal channel
- Deferred cleanup: `defer tracer.Stop()`, `defer database.Close()`
- Representative snippet:

```go
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()

	tracer.Start(
		tracer.WithService(config.Envs.DDService),
		tracer.WithEnv(config.Envs.DDEnv),
	)
	defer tracer.Stop()
}
```
