# agentic_instructions.md

## Purpose
User authentication, route registration, and PostgreSQL user storage. Registers core routes (login, register, downtimes, events, hosts, private locations) and handles user CRUD.

## Technology
Go, net/http, database/sql, encoding/json

## Contents
- `routes.go` -- Handler with RegisterRoutes (registers core routes), HandleLogin, HandleRegister
- `storage.go` -- Storage with GetUserbyUUID, CreateUser, ScanRowsIntoUser
- `routes_test.go` -- Tests (not examined)

## Key Functions
- `NewHandler(storage) *Handler` -- Creates user handler
- `(h *Handler) RegisterRoutes(router)` -- Registers /login, /register, /v1/downtimes, /v1/events, /v1/hosts/active, /v1/pl/refresh/{name}
- `(h *Handler) HandleLogin(w, r) (int, any)` -- Authenticates user by UUID
- `(h *Handler) HandleRegister(w, r) (int, any)` -- Creates new user (checks for duplicate UUID)
- `NewStorage(db) *Storage` -- Creates user storage
- `(s *Storage) GetUserbyUUID(uuid int) (*types.User, error)` -- Retrieves user by UUID
- `(s *Storage) CreateUser(name, uuid) (*types.User, error)` -- Inserts new user

## Data Types
- `Handler` -- struct: storage types.UserStorage (interface)
- `Storage` -- struct: db *sql.DB
- Uses `types.User` and `types.RegisterUserPayload` from cmd/types

## Logging
Uses `log.Printf` for "User Not Found" warnings

## CRUD Entry Points
- **Create**: POST /register with RegisterUserPayload
- **Read**: POST /login with UUID lookup
- **Update**: N/A
- **Delete**: N/A

## Style Guide
- Route registration as a method on Handler (RegisterRoutes pattern)
- Uses types.UserStorage interface for testability
- PostgreSQL $1 placeholders
- Representative snippet:

```go
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) (int, any) {
	var payload types.RegisterUserPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return http.StatusBadRequest, map[string]string{"error": "invalid request body"}
	}

	existing, _ := h.storage.GetUserbyUUID(payload.UUID)
	if existing != nil && existing.UUID != 0 {
		return http.StatusConflict, map[string]string{"error": "user with UUID already exists"}
	}

	user, err := h.storage.CreateUser(payload.Name, payload.UUID)
	if err != nil {
		return http.StatusInternalServerError, map[string]string{"error": "failed to create user"}
	}
	return http.StatusCreated, user
}
```
