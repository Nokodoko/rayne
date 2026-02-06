package rum

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// RUM Diagnostic Tests
// ============================================================================
//
// These tests diagnose why unique visitors are NOT showing up in Datadog's
// commercial domain (datadoghq.com). They are designed to FAIL against the
// current misconfigured codebase and document exactly what needs to change.
//
// Run with: go test -v -run Test_ ./services/rum/
//
// Summary of findings:
//   1. Frontend RUM SDK targets ddog-gov.datadoghq.com (gov) not datadoghq.com (commercial)
//   2. Backend API URLs in urls.go target api.ddog-gov.com (gov)
//   3. No DD_SITE env var in config -- site domain is hardcoded everywhere
//   4. Helm values.yaml is correct (datadoghq.com) but conflicts with code
//   5. InitVisitor handler works correctly (returns valid UUID)
//   6. DD_RUM.setUser() is called correctly with visitor UUID
//   7. allowedTracingUrls does NOT include n0kos.com -- breaks RUM-APM in prod
//   8. Frontend sends "page_view" but backend only accepts "view" -- all page
//      views silently dropped with 400 Bad Request
// ============================================================================

// projectRoot returns the absolute path to the rayne project root directory.
// It works by finding the directory of this test file (which lives at
// mkii_ddog_server/services/rum/) and walking up to reach the project root.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot determine project root")
	}
	// thisFile = .../rayne/mkii_ddog_server/services/rum/handler_diagnostic_test.go
	// Go up 4 levels: rum -> services -> mkii_ddog_server -> rayne
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	return root
}

// ---------------------------------------------------------------------------
// Test 1: Frontend RUM SDK must target the commercial Datadog site
// ---------------------------------------------------------------------------
//
// CURRENT STATE: The JS file has site: 'ddog-gov.datadoghq.com' which sends
// RUM data to the US Government cloud. Visitors will never appear on the
// commercial datadoghq.com dashboard.
//
// FIX: Change site to 'datadoghq.com' in frontend/static/js/datadog-rum-init.js
func Test_RUMConfigSiteIsCommercial(t *testing.T) {
	root := projectRoot(t)
	jsPath := filepath.Join(root, "frontend", "static", "js", "datadog-rum-init.js")

	data, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("Cannot read frontend RUM JS file at %s: %v", jsPath, err)
	}

	content := string(data)

	// The JS must contain site: 'datadoghq.com' (commercial)
	if !strings.Contains(content, "site: 'datadoghq.com'") {
		// Identify what it currently has
		re := regexp.MustCompile(`site:\s*['"]([^'"]+)['"]`)
		match := re.FindStringSubmatch(content)
		currentSite := "<not found>"
		if len(match) > 1 {
			currentSite = match[1]
		}
		t.Errorf(
			"Frontend RUM SDK site is '%s' but must be 'datadoghq.com' (commercial).\n"+
				"File: %s\n"+
				"FIX: Change site: '%s' to site: 'datadoghq.com' in the RUM_CONFIG object.\n"+
				"RUM data sent to ddog-gov.datadoghq.com will never appear in the commercial Datadog dashboard.",
			currentSite, jsPath, currentSite,
		)
	}

	// Also verify it does NOT contain the gov site
	if strings.Contains(content, "ddog-gov.datadoghq.com") {
		t.Errorf(
			"Frontend RUM JS still references 'ddog-gov.datadoghq.com'.\n"+
				"File: %s\n"+
				"FIX: Replace ALL occurrences of 'ddog-gov.datadoghq.com' with 'datadoghq.com'.",
			jsPath,
		)
	}
}

// ---------------------------------------------------------------------------
// Test 2: Backend API URLs for RUM must target the commercial API
// ---------------------------------------------------------------------------
//
// CURRENT STATE: All URLs in cmd/utils/urls/urls.go use api.ddog-gov.com
// (US Government cloud). The RUM applications endpoint will query the wrong
// cloud, returning no data or 403 errors on the commercial side.
//
// FIX: Change RUMApplications (and ideally all URLs) to use api.datadoghq.com,
// or make the base URL configurable via DD_SITE env var.
func Test_RUMApiUrlsAreCommercial(t *testing.T) {
	root := projectRoot(t)
	urlsPath := filepath.Join(root, "mkii_ddog_server", "cmd", "utils", "urls", "urls.go")

	data, err := os.ReadFile(urlsPath)
	if err != nil {
		t.Fatalf("Cannot read URLs file at %s: %v", urlsPath, err)
	}

	content := string(data)

	// Find all RUM-related URLs
	rumRe := regexp.MustCompile(`(?i)rum.*"(https://[^"]+)"`)
	rumMatches := rumRe.FindAllStringSubmatch(content, -1)

	if len(rumMatches) == 0 {
		t.Fatal("No RUM-related URLs found in urls.go. Expected at least RUMApplications.")
	}

	for _, match := range rumMatches {
		url := match[1]
		if strings.Contains(url, "ddog-gov.com") {
			t.Errorf(
				"RUM API URL uses US Gov cloud: %s\n"+
					"File: %s\n"+
					"FIX: Change 'api.ddog-gov.com' to 'api.datadoghq.com' for commercial Datadog,\n"+
					"or make the base URL configurable via a DD_SITE environment variable.",
				url, urlsPath,
			)
		}
	}

	// Also flag ALL gov URLs since they indicate a systemic misconfiguration
	govCount := strings.Count(content, "api.ddog-gov.com")
	commercialCount := strings.Count(content, "api.datadoghq.com")
	if govCount > 0 {
		t.Errorf(
			"urls.go has %d URL(s) targeting api.ddog-gov.com and %d targeting api.datadoghq.com.\n"+
				"File: %s\n"+
				"FIX: If you want to use the commercial Datadog cloud, change ALL URLs from\n"+
				"'api.ddog-gov.com' to 'api.datadoghq.com', or make the base domain configurable\n"+
				"so the same code works for both gov and commercial deployments.",
			govCount, commercialCount, urlsPath,
		)
	}
}

// ---------------------------------------------------------------------------
// Test 3: DD_SITE env var must exist in the Config struct
// ---------------------------------------------------------------------------
//
// CURRENT STATE: cmd/config/env.go has no DD_SITE field. The Datadog site
// domain is hardcoded in multiple places (JS, Go URLs). Without a DD_SITE
// config field, switching between gov and commercial requires editing source
// code instead of setting an environment variable.
//
// FIX: Add DDSite string field to Config struct, populated from DD_SITE env
// var with default "datadoghq.com".
func Test_DDSiteEnvVarExists(t *testing.T) {
	root := projectRoot(t)
	envPath := filepath.Join(root, "mkii_ddog_server", "cmd", "config", "env.go")

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Cannot read config file at %s: %v", envPath, err)
	}

	content := string(data)

	// Check for a DD_SITE field in the struct
	if !strings.Contains(content, "DDSite") && !strings.Contains(content, "DD_SITE") {
		t.Errorf(
			"Config struct in env.go has no DD_SITE field.\n"+
				"File: %s\n"+
				"Current Config fields: PublicHost, DB*, DDService, DDEnv, DDVersion, DDAgentHost\n"+
				"FIX: Add the following to the Config struct and initConfig():\n\n"+
				"  // In Config struct:\n"+
				"  DDSite string\n\n"+
				"  // In initConfig():\n"+
				"  DDSite: utils.GetEnv(\"DD_SITE\", \"datadoghq.com\"),\n\n"+
				"Then use config.Envs.DDSite to build API URLs dynamically instead of hardcoding domains.",
			envPath,
		)
	}

	// Even if DD_SITE exists as text, verify it defaults to commercial
	if strings.Contains(content, "DD_SITE") && !strings.Contains(content, "datadoghq.com") {
		t.Errorf(
			"DD_SITE is referenced in env.go but does not default to 'datadoghq.com'.\n"+
				"File: %s\n"+
				"FIX: Set the default value to 'datadoghq.com':\n"+
				"  DDSite: utils.GetEnv(\"DD_SITE\", \"datadoghq.com\"),",
			envPath,
		)
	}
}

// ---------------------------------------------------------------------------
// Test 4: Helm values must specify the commercial Datadog site
// ---------------------------------------------------------------------------
//
// CURRENT STATE: helm/values.yaml has site: datadoghq.com (correct for
// commercial). This test verifies that it stays correct and does not regress
// back to the gov site.
func Test_HelmValuesMatchCommercialSite(t *testing.T) {
	root := projectRoot(t)
	helmPath := filepath.Join(root, "helm", "values.yaml")

	data, err := os.ReadFile(helmPath)
	if err != nil {
		t.Fatalf("Cannot read Helm values file at %s: %v", helmPath, err)
	}

	content := string(data)

	// Verify site: datadoghq.com exists
	siteRe := regexp.MustCompile(`(?m)^\s*site:\s*(.+)$`)
	match := siteRe.FindStringSubmatch(content)
	if len(match) < 2 {
		t.Fatalf(
			"No 'site:' field found in Helm values.yaml.\n"+
				"File: %s\n"+
				"FIX: Add 'site: datadoghq.com' under the datadog: section.",
			helmPath,
		)
	}

	siteValue := strings.TrimSpace(match[1])
	if siteValue != "datadoghq.com" {
		t.Errorf(
			"Helm values.yaml has site: %s (expected datadoghq.com).\n"+
				"File: %s\n"+
				"FIX: Change site value to 'datadoghq.com' for commercial Datadog.",
			siteValue, helmPath,
		)
	}

	// Verify there is no gov reference in the helm values
	if strings.Contains(content, "ddog-gov") {
		t.Errorf(
			"Helm values.yaml contains a reference to 'ddog-gov', which is the US Gov cloud.\n"+
				"File: %s\n"+
				"FIX: Remove or replace all 'ddog-gov' references with commercial equivalents.",
			helmPath,
		)
	}
}

// ---------------------------------------------------------------------------
// Test 5: InitVisitor handler must return a valid UUID
// ---------------------------------------------------------------------------
//
// This test uses a no-op SQL driver to satisfy the *sql.DB dependency and
// verifies the InitVisitor handler produces a response containing a valid
// UUID v4 visitor_uuid field.
func Test_InitVisitorReturnsValidUUID(t *testing.T) {
	db := newNoOpDB(t)
	defer db.Close()

	storage := NewStorage(db)
	handler := NewHandler(storage)

	body := `{"user_agent":"TestBot/1.0","page_url":"https://n0kos.com/"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rum/init", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	statusCode, result := handler.InitVisitor(w, req)

	// The handler generates a UUID and then tries to store it. With our
	// no-op driver the storage succeeds (returns nil error). We should get
	// a 201 Created with a valid UUID.
	if statusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d. Response: %+v",
			http.StatusCreated, statusCode, result)
	}

	resp, ok := result.(VisitorInitResponse)
	if !ok {
		t.Fatalf("Response is not VisitorInitResponse, got %T: %+v", result, result)
	}

	// Validate UUID format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	uuidRe := regexp.MustCompile(
		`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	if !uuidRe.MatchString(resp.VisitorUUID) {
		t.Errorf(
			"VisitorUUID '%s' is not a valid UUID.\n"+
				"Expected format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
			resp.VisitorUUID,
		)
	}

	if !uuidRe.MatchString(resp.SessionID) {
		t.Errorf(
			"SessionID '%s' is not a valid UUID.\n"+
				"Expected format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
			resp.SessionID,
		)
	}

	if !resp.IsNew {
		t.Error("Expected IsNew=true for a brand new visitor, got false")
	}
}

// ---------------------------------------------------------------------------
// Test 6: Frontend JS must call DD_RUM.setUser() with the visitor UUID
// ---------------------------------------------------------------------------
//
// CURRENT STATE: The JS does call DD_RUM.setUser() with the visitor UUID.
// This test verifies that integration is present and correctly shaped.
func Test_SetUserCalledWithVisitorUUID(t *testing.T) {
	root := projectRoot(t)
	jsPath := filepath.Join(root, "frontend", "static", "js", "datadog-rum-init.js")

	data, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("Cannot read frontend RUM JS file at %s: %v", jsPath, err)
	}

	content := string(data)

	// Verify DD_RUM.setUser is called
	if !strings.Contains(content, "DD_RUM.setUser") && !strings.Contains(content, "setUser(") {
		t.Errorf(
			"Frontend JS does not call DD_RUM.setUser().\n"+
				"File: %s\n"+
				"FIX: After calling DD_RUM.init(config), call:\n"+
				"  DD_RUM.setUser({ id: visitorData.visitor_uuid })\n"+
				"This ties RUM sessions to unique visitor UUIDs in the Datadog dashboard.",
			jsPath,
		)
	}

	// Verify the setUser call includes the visitor_uuid as the id field
	setUserRe := regexp.MustCompile(
		`setUser\s*\(\s*\{[^}]*id\s*:\s*visitorData\.visitor_uuid`)
	if !setUserRe.MatchString(content) {
		// Looser check: at least visitor_uuid is referenced near setUser
		if !strings.Contains(content, "setUser") ||
			!strings.Contains(content, "visitor_uuid") {
			t.Errorf(
				"DD_RUM.setUser() does not set id to the visitor UUID.\n"+
					"File: %s\n"+
					"FIX: Ensure setUser includes: { id: visitorData.visitor_uuid }",
				jsPath,
			)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 7: allowedTracingUrls must include n0kos.com
// ---------------------------------------------------------------------------
//
// CURRENT STATE: allowedTracingUrls only matches /localhost/ and /rayne/.
// Requests to n0kos.com will NOT have Datadog trace context injected, so
// RUM-APM correlation will not work in production.
//
// FIX: Add { match: /n0kos\.com/, propagatorTypes: ['datadog'] } to
// allowedTracingUrls in the RUM_CONFIG.
func Test_AllowedTracingUrlsIncludesDomain(t *testing.T) {
	root := projectRoot(t)
	jsPath := filepath.Join(root, "frontend", "static", "js", "datadog-rum-init.js")

	data, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("Cannot read frontend RUM JS file at %s: %v", jsPath, err)
	}

	content := string(data)

	// Check that allowedTracingUrls exists
	if !strings.Contains(content, "allowedTracingUrls") {
		t.Fatalf(
			"Frontend JS does not have allowedTracingUrls in the RUM config.\n"+
				"File: %s\n"+
				"FIX: Add allowedTracingUrls to RUM_CONFIG to enable RUM-APM trace correlation.",
			jsPath,
		)
	}

	// Check that n0kos.com is included in the tracing URLs
	if !strings.Contains(content, "n0kos") {
		t.Errorf(
			"allowedTracingUrls does not include 'n0kos.com'.\n"+
				"File: %s\n"+
				"Current allowedTracingUrls only matches /localhost/ and /rayne/.\n"+
				"FIX: Add the production domain to allowedTracingUrls:\n"+
				"  allowedTracingUrls: [\n"+
				"    { match: /localhost/, propagatorTypes: ['datadog'] },\n"+
				"    { match: /rayne/, propagatorTypes: ['datadog'] },\n"+
				"    { match: /n0kos\\.com/, propagatorTypes: ['datadog'] }\n"+
				"  ]\n"+
				"Without this, trace context headers will NOT be injected on requests to n0kos.com,\n"+
				"breaking RUM-APM correlation in production.",
			jsPath,
		)
	}
}

// ---------------------------------------------------------------------------
// Test 8: Backend must accept "page_view" as a valid event_type
// ---------------------------------------------------------------------------
//
// CURRENT STATE: The frontend JS sends event_type: 'page_view' in the
// trackPageView function, but the backend TrackEvent handler only accepts
// these event types: "view", "action", "error", "resource", "long_task".
//
// "page_view" is NOT in the valid set, so every page view tracked by the
// frontend returns 400 Bad Request with "invalid event_type". This means
// page views are silently dropped.
//
// FIX: Either:
//
//	(a) Add "page_view" to the validTypes map in handler.go, or
//	(b) Change the frontend JS to send event_type: "view" instead of "page_view"
func Test_EventTypePageViewAccepted(t *testing.T) {
	// The validation check in TrackEvent happens BEFORE any storage calls,
	// so nil DB is fine -- the handler returns 400 before touching the DB.
	storage := &Storage{db: nil}
	handler := NewHandler(storage)

	// Simulate exactly what the frontend sends (see datadog-rum-init.js line 122)
	trackReq := TrackEventRequest{
		VisitorUUID: "test-uuid-1234-5678-abcd-ef0123456789",
		SessionID:   "test-sess-1234-5678-abcd-ef0123456789",
		EventType:   "page_view",
		PageURL:     "https://n0kos.com/",
		PageTitle:   "Rayne Portfolio",
	}

	body, _ := json.Marshal(trackReq)
	req := httptest.NewRequest(http.MethodPost, "/v1/rum/track", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	statusCode, result := handler.TrackEvent(w, req)

	if statusCode == http.StatusBadRequest {
		errMap, ok := result.(map[string]string)
		errMsg := ""
		if ok {
			errMsg = errMap["error"]
		}
		t.Errorf(
			"Backend rejects 'page_view' event_type with 400 Bad Request: %q\n"+
				"The frontend JS (datadog-rum-init.js line ~122) sends event_type: 'page_view',\n"+
				"but handler.go only accepts: view, action, error, resource, long_task.\n\n"+
				"EVERY page view from the frontend is silently dropped.\n\n"+
				"FIX (option A - backend): Add 'page_view' to the validTypes map in handler.go:\n"+
				"  validTypes := map[string]bool{\n"+
				"    \"view\":      true,\n"+
				"    \"page_view\": true,  // <-- add this\n"+
				"    \"action\":    true,\n"+
				"    ...\n"+
				"  }\n\n"+
				"FIX (option B - frontend): Change event_type from 'page_view' to 'view' in\n"+
				"  the trackPageView() function in datadog-rum-init.js.",
			errMsg,
		)
		return
	}

	// If we somehow get past validation (i.e., the fix has been applied),
	// we might panic on nil DB. That is fine -- it means the validation
	// is fixed. We only care that it did NOT return 400.
	if statusCode != http.StatusAccepted && statusCode != http.StatusInternalServerError {
		t.Errorf("Expected status %d for page_view event, got %d. Response: %+v",
			http.StatusAccepted, statusCode, result)
	}
}

// ============================================================================
// No-op SQL driver for handler tests
// ============================================================================
//
// This registers a minimal SQL driver that accepts all queries and returns
// empty results with no errors. It allows handler tests to exercise the full
// code path without requiring a real PostgreSQL connection.

func init() {
	sql.Register("noop_rum_test", &noOpDriver{})
}

func newNoOpDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("noop_rum_test", "")
	if err != nil {
		t.Fatalf("Failed to open no-op database: %v", err)
	}
	return db
}

// noOpDriver implements database/sql/driver interfaces with no-op behavior.
type noOpDriver struct{}

func (d *noOpDriver) Open(_ string) (driver.Conn, error) {
	return &noOpConn{}, nil
}

type noOpConn struct{}

func (c *noOpConn) Prepare(query string) (driver.Stmt, error) {
	return &noOpStmt{}, nil
}

func (c *noOpConn) Close() error { return nil }

func (c *noOpConn) Begin() (driver.Tx, error) {
	return &noOpTx{}, nil
}

type noOpTx struct{}

func (tx *noOpTx) Commit() error   { return nil }
func (tx *noOpTx) Rollback() error { return nil }

type noOpStmt struct{}

func (s *noOpStmt) Close() error { return nil }

func (s *noOpStmt) NumInput() int { return -1 } // -1 means the driver does not check

func (s *noOpStmt) Exec(args []driver.Value) (driver.Result, error) {
	return &noOpResult{}, nil
}

func (s *noOpStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &noOpRows{}, nil
}

type noOpResult struct{}

func (r *noOpResult) LastInsertId() (int64, error) { return 0, nil }
func (r *noOpResult) RowsAffected() (int64, error) { return 1, nil }

// noOpRows returns no rows. This causes QueryRow().Scan() to return
// sql.ErrNoRows, which the handler interprets as "no existing visitor found".
type noOpRows struct{}

func (r *noOpRows) Columns() []string {
	return []string{"id", "uuid", "first_seen", "last_seen", "session_count",
		"total_views", "user_agent", "ip_hash", "country", "city"}
}

func (r *noOpRows) Close() error              { return nil }
func (r *noOpRows) Next(dest []driver.Value) error { return io.EOF }
