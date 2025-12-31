package api

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils"
	"github.com/Nokodoko/mkii_ddog_server/services/catalog"
	"github.com/Nokodoko/mkii_ddog_server/services/demo"
	"github.com/Nokodoko/mkii_ddog_server/services/downtimes"
	"github.com/Nokodoko/mkii_ddog_server/services/events"
	"github.com/Nokodoko/mkii_ddog_server/services/hosts"
	"github.com/Nokodoko/mkii_ddog_server/services/logs"
	"github.com/Nokodoko/mkii_ddog_server/services/monitors"
	"github.com/Nokodoko/mkii_ddog_server/services/pl"
	"github.com/Nokodoko/mkii_ddog_server/services/rum"
	"github.com/Nokodoko/mkii_ddog_server/services/user"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

type DDogServer struct {
	addr string
	db   *sql.DB
}

func NewDdogServer(addr string, db *sql.DB) *DDogServer {
	return &DDogServer{
		addr: addr,
		db:   db,
	}
}

func (d *DDogServer) Run() error {
	router := http.NewServeMux()

	// Initialize storages
	userStorage := user.NewStorage(d.db)
	webhookStorage := webhooks.NewStorage(d.db)
	rumStorage := rum.NewStorage(d.db)

	// Initialize webhook services
	downtimeSvc := webhooks.NewDowntimeService()
	webhookProcessor := webhooks.NewProcessor(webhookStorage, downtimeSvc)

	// Initialize handlers
	userHandler := user.NewHandler(userStorage)
	webhookHandler := webhooks.NewHandler(webhookStorage, webhookProcessor)
	rumHandler := rum.NewHandler(rumStorage)
	demoHandler := demo.NewHandler(webhookStorage, rumStorage)

	// Initialize database tables for new services
	if err := webhookStorage.InitTables(); err != nil {
		log.Printf("Warning: Failed to initialize webhook tables: %v", err)
	}
	if err := rumStorage.InitTables(); err != nil {
		log.Printf("Warning: Failed to initialize RUM tables: %v", err)
	}

	// Register routes

	// Health check
	utils.Endpoint(router, "GET", "/health", func(w http.ResponseWriter, r *http.Request) (int, any) {
		return http.StatusOK, map[string]string{"status": "healthy"}
	})

	// User routes
	utils.Endpoint(router, "POST", "/login", userHandler.HandleLogin)
	utils.Endpoint(router, "POST", "/register", userHandler.HandleRegister)

	// Downtimes
	utils.Endpoint(router, "GET", "/v1/downtimes", downtimes.GetDowntimes)

	// Events
	utils.Endpoint(router, "GET", "/v1/events", events.GetEvents)

	// Hosts
	utils.Endpoint(router, "GET", "/v1/hosts", hosts.GetHosts)
	utils.Endpoint(router, "GET", "/v1/hosts/active", hosts.GetTotalActiveHosts)
	utils.EndpointWithPathParams(router, "GET", "/v1/hosts/{hostname}/tags", "hostname", hosts.GetHostTagsHandler)
	utils.Endpoint(router, "GET", "/v1/hosts/tags", hosts.GetAllHostsTags)

	// Private Locations
	utils.EndpointWithPathParams(router, "GET", "/v1/pl/refresh/{name}", "name", pl.ImageRotation)

	// Webhooks
	utils.Endpoint(router, "POST", "/v1/webhooks/receive", webhookHandler.ReceiveWebhook)
	utils.Endpoint(router, "GET", "/v1/webhooks/events", webhookHandler.GetWebhookEvents)
	utils.EndpointWithPathParams(router, "GET", "/v1/webhooks/events/{id}", "id", webhookHandler.GetWebhookEvent)
	utils.EndpointWithPathParams(router, "GET", "/v1/webhooks/monitor/{monitorId}", "monitorId", webhookHandler.GetEventsByMonitor)
	utils.Endpoint(router, "POST", "/v1/webhooks/create", webhookHandler.CreateWebhook)
	utils.Endpoint(router, "POST", "/v1/webhooks/config", webhookHandler.SaveWebhookConfig)
	utils.Endpoint(router, "GET", "/v1/webhooks/config", webhookHandler.GetWebhookConfigs)
	utils.Endpoint(router, "GET", "/v1/webhooks/stats", webhookHandler.GetWebhookStats)
	utils.Endpoint(router, "POST", "/v1/webhooks/reprocess", webhookHandler.ReprocessPending)

	// RUM (Real User Monitoring)
	utils.Endpoint(router, "POST", "/v1/rum/init", rumHandler.InitVisitor)
	utils.Endpoint(router, "POST", "/v1/rum/track", rumHandler.TrackEvent)
	utils.Endpoint(router, "POST", "/v1/rum/session/end", rumHandler.EndSession)
	utils.EndpointWithPathParams(router, "GET", "/v1/rum/visitor/{uuid}", "uuid", rumHandler.GetVisitor)
	utils.EndpointWithPathParams(router, "GET", "/v1/rum/session/{sessionId}", "sessionId", rumHandler.GetSession)
	utils.Endpoint(router, "GET", "/v1/rum/visitors", rumHandler.GetUniqueVisitors)
	utils.Endpoint(router, "GET", "/v1/rum/analytics", rumHandler.GetAnalytics)
	utils.Endpoint(router, "GET", "/v1/rum/sessions", rumHandler.GetRecentSessions)

	// Demo data generators
	utils.Endpoint(router, "POST", "/v1/demo/seed/webhooks", demoHandler.SeedWebhookEvents)
	utils.Endpoint(router, "POST", "/v1/demo/seed/rum", demoHandler.SeedRUMData)
	utils.Endpoint(router, "POST", "/v1/demo/seed/all", demoHandler.SeedAllData)
	utils.Endpoint(router, "GET", "/v1/demo/monitors", demoHandler.GenerateSampleMonitors)
	utils.Endpoint(router, "GET", "/v1/demo/status", demoHandler.GetDemoStatus)

	// Monitors
	utils.Endpoint(router, "GET", "/v1/monitors", monitors.ListMonitors)
	utils.Endpoint(router, "GET", "/v1/monitors/triggered", monitors.GetTriggeredMonitors)
	utils.Endpoint(router, "GET", "/v1/monitors/ids", monitors.GetMonitorIDs)
	utils.Endpoint(router, "GET", "/v1/monitors/pages", monitors.GetMonitorPageCount)
	utils.EndpointWithPathParams(router, "GET", "/v1/monitors/{id}", "id", monitors.GetMonitorByID)

	// Logs
	utils.Endpoint(router, "POST", "/v1/logs/search", logs.SearchLogs)
	utils.Endpoint(router, "POST", "/v1/logs/search/advanced", logs.SearchLogsAdvanced)

	// Service Catalog
	utils.Endpoint(router, "GET", "/v1/services", catalog.ListServices)
	utils.Endpoint(router, "POST", "/v1/services/definitions", catalog.CreateServiceDefinition)
	utils.Endpoint(router, "POST", "/v1/services/definitions/advanced", catalog.CreateServiceDefinitionAdvanced)

	log.Printf(`
		.__.  ._  _
		|(_|\/| |(/_
		    /      ...agentic solutions
		Listening on %s

		Available endpoints:
		  GET  /health
		  POST /login, /register
		  GET  /v1/downtimes, /v1/events
		  GET  /v1/hosts, /v1/hosts/active, /v1/hosts/{hostname}/tags
		  GET  /v1/pl/refresh/{name}
		  POST /v1/webhooks/receive, /v1/webhooks/create
		  GET  /v1/webhooks/events, /v1/webhooks/stats
		  POST /v1/rum/init, /v1/rum/track
		  GET  /v1/rum/analytics, /v1/rum/visitors
		  GET  /v1/monitors, /v1/monitors/triggered, /v1/monitors/{id}
		  POST /v1/logs/search
		  GET  /v1/services
		  POST /v1/services/definitions
		  POST /v1/demo/seed/all
	`, d.addr)
	return http.ListenAndServe(d.addr, router)
}

// log.Println("\x1B[3m" + green + "DB: Successfully Connected" + "\x1B[0m")
// ┏━╸┏━┓   ╺┳┓╺┳┓┏━┓┏━╸   ┏━┓┏━╸┏━┓╻ ╻┏━╸┏━┓
// ┣╸ ┗━┓    ┃┃ ┃┃┃ ┃┃╺┓   ┗━┓┣╸ ┣┳┛┃┏┛┣╸ ┣┳┛
// ╹  ┗━┛   ╺┻┛╺┻┛┗━┛┗━┛   ┗━┛┗━╸╹┗╸┗┛ ┗━╸╹┗╸
