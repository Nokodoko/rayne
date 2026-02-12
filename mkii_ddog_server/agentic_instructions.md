# agentic_instructions.md

## Purpose
Go REST API server root. Contains the build system, Docker configuration, Go module definition, and vendor dependencies for the Rayne Datadog API wrapper.

## Technology
Go 1.24, Docker (multi-stage alpine), Make

## Contents
- `Makefile` -- Build targets: r (run), build, test, curl shortcuts for endpoint testing
- `Dockerfile` -- Multi-stage build: golang:1.24-alpine builder -> alpine:3.19 runtime, non-root user
- `go.mod` / `go.sum` -- Go module: github.com/Nokodoko/mkii_ddog_server
- `cmd/` -- Application code (api, config, db, migrate, types, utils)
- `services/` -- Feature modules (webhooks, agents, rum, demo, accounts, etc.)
- `vendor/` -- Vendored dependencies (excluded from documentation)
- `bin/` -- Build output (excluded from documentation)

## Key Functions
N/A (build configuration only)

## Data Types
N/A

## Logging
N/A

## CRUD Entry Points
- **Create**: `make build` compiles binary to bin/fsDDServer
- **Read**: `make r` runs the server, `make test` runs all tests
- **Update**: Edit go.mod for dependencies, Makefile for build targets
- **Delete**: N/A

## Style Guide
- Entry point: `cmd/migrate/main.go`
- Binary name: `rayne` (in Docker), `fsDDServer` (local build)
- Docker: non-root user (rayne:1000), health check via wget
- Representative Makefile snippet:

```makefile
r:
	go run cmd/migrate/main.go

test:
	@go test -v ./...

build:
	@go build -o bin/fsDDServer cmd/main.go
```
