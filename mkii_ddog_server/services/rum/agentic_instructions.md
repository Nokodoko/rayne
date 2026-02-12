# agentic_instructions.md

## Purpose
Real User Monitoring (RUM) visitor tracking system. Manages visitor UUIDs, sessions, and page-level events with PostgreSQL storage and APM trace correlation.

## Technology
Go, net/http, database/sql, encoding/json, crypto/sha256, github.com/google/uuid, dd-trace-go

## Contents
- `handler.go` -- HTTP handlers for visitor init, event tracking, session management, analytics
- `types.go` -- Visitor, Session, RUMEvent, request/response types, VisitorAnalytics
- `storage.go` -- PostgreSQL storage (rum_visitors, rum_sessions, rum_events tables) with analytics queries

## Key Functions
- `NewHandler(storage) *Handler` -- Creates RUM handler
- `(h *Handler) InitVisitor(w, r) (int, any)` -- Creates/resumes visitor with UUID, creates session, returns APM trace context
- `(h *Handler) TrackEvent(w, r) (int, any)` -- Records RUM event (view, action, error, resource, long_task) with RUM-APM correlation
- `(h *Handler) EndSession(w, r) (int, any)` -- Marks session ended, calculates duration
- `(h *Handler) GetAnalytics(w, r) (int, any)` -- Returns comprehensive analytics (visitors, sessions, pages, devices, browsers)
- `(h *Handler) GetRecentSessions(w, r) (int, any)` -- Paginated session list
- `(s *Storage) InitTables() error` -- Creates rum_visitors, rum_sessions, rum_events tables with indexes
- `(s *Storage) CreateVisitor(uuid, userAgent, ipHash) error` -- Creates visitor record
- `(s *Storage) StoreEvent(event) error` -- Stores event, auto-increments page views for view events
- `(s *Storage) GetAnalytics(from, to) (*VisitorAnalytics, error)` -- Aggregates analytics for time range

## Data Types
- `Visitor` -- struct: ID, UUID, FirstSeen, LastSeen, SessionCount, TotalViews, UserAgent, IPHash, Country, City
- `Session` -- struct: ID, VisitorUUID, SessionID, StartTime, EndTime, PageViews, DurationMs, Referrer, EntryPage, ExitPage, DeviceType, Browser, OS
- `RUMEvent` -- struct: ID, VisitorUUID, SessionID, EventType, Timestamp, PageURL, PageTitle, ActionName, ActionType, ErrorMsg, Duration, Metadata (JSONB)
- `VisitorInitRequest` -- struct: ExistingUUID, VisitorUUID (alias), UserAgent, Referrer, EntryPage, PageURL (alias)
- `VisitorInitResponse` -- struct: VisitorUUID, SessionID, IsNew, Message, TraceID, SpanID
- `VisitorAnalytics` -- struct: UniqueVisitors, TotalSessions, TotalPageViews, AvgSessionDuration, NewVisitors, ReturningVisitors, TopPages, ByDevice, ByBrowser

## Logging
None (errors returned to callers)

## CRUD Entry Points
- **Create**: `InitVisitor` creates visitors and sessions; `TrackEvent` creates events
- **Read**: `GetAnalytics`, `GetRecentSessions`, `GetVisitor`, `GetSession`
- **Update**: `EndSession` marks sessions ended; `UpdateVisitorLastSeen` increments session count
- **Delete**: Cascading deletes via PostgreSQL foreign keys (visitor UUID)

## Style Guide
- Privacy-first: IP addresses hashed with SHA-256, never stored raw
- APM-RUM correlation: trace_id and span_id extracted from request context and included in responses
- Time range parsing supports RFC3339, date-only, and period shortcuts (1h, 6h, 24h, 7d, 30d)
- Representative snippet:

```go
func (h *Handler) InitVisitor(w http.ResponseWriter, r *http.Request) (int, any) {
	var req VisitorInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ExistingUUID = r.Header.Get("X-Visitor-UUID")
		req.UserAgent = r.UserAgent()
	}

	traceID, spanID := getTraceContext(r)

	if req.ExistingUUID != "" {
		visitor, err := h.storage.GetVisitorByUUID(req.ExistingUUID)
		if err == nil && visitor != nil {
			sessionID := uuid.New().String()
			h.storage.UpdateVisitorLastSeen(req.ExistingUUID)
			h.storage.CreateSession(req.ExistingUUID, sessionID, req.Referrer, req.EntryPage, req.UserAgent)
			return http.StatusOK, VisitorInitResponse{
				VisitorUUID: req.ExistingUUID, SessionID: sessionID,
				IsNew: false, TraceID: traceID, SpanID: spanID,
			}
		}
	}
	// ... create new visitor
}
```
