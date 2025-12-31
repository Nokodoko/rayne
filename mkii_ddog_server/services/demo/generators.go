package demo

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/Nokodoko/mkii_ddog_server/services/rum"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// Seed data for generating fake data
var (
	services = []string{
		"web-frontend", "api-gateway", "auth-service", "payment-service",
		"user-service", "notification-service", "inventory-service",
		"order-service", "analytics-service", "search-service",
	}

	alertStatuses = []string{"Alert", "OK", "Warn", "No Data"}
	priorities    = []string{"normal", "low"}
	eventTypes    = []string{"view", "action", "error", "resource"}

	pagePaths = []string{
		"/", "/home", "/dashboard", "/products", "/products/123",
		"/cart", "/checkout", "/profile", "/settings", "/about",
		"/contact", "/help", "/faq", "/login", "/register",
	}

	pageTitles = []string{
		"Home", "Dashboard", "Products", "Product Details",
		"Shopping Cart", "Checkout", "Profile", "Settings",
		"About Us", "Contact", "Help Center", "FAQ", "Login", "Register",
	}

	referrers = []string{
		"https://google.com", "https://facebook.com", "https://twitter.com",
		"https://linkedin.com", "", "direct", "https://reddit.com",
	}

	userAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile",
		"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile",
		"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	}

	monitorNames = []string{
		"CPU Usage High", "Memory Usage Critical", "Disk Space Low",
		"API Latency Alert", "Error Rate Spike", "Request Queue Full",
		"Database Connection Pool Exhausted", "SSL Certificate Expiring",
		"Container Restart Loop", "Network Latency High",
		"HTTP 5xx Error Rate", "Service Unavailable", "Health Check Failed",
	}

	hostnames = []string{
		"web-prod-01", "web-prod-02", "api-prod-01", "api-prod-02",
		"db-primary", "db-replica-01", "cache-01", "worker-01",
		"scheduler-01", "lb-01", "monitoring-01",
	}

	scopes = []string{
		"env:production", "env:staging", "env:development",
		"team:platform", "team:frontend", "team:backend",
		"region:us-east-1", "region:us-west-2", "region:eu-west-1",
	}
)

// GenerateWebhookPayload creates a fake webhook payload
func GenerateWebhookPayload() webhooks.WebhookPayload {
	now := time.Now()
	alertStatus := alertStatuses[rand.Intn(len(alertStatuses))]

	return webhooks.WebhookPayload{
		AlertID:      rand.Int63n(1000000) + 1,
		AlertTitle:   fmt.Sprintf("[%s] %s", alertStatus, monitorNames[rand.Intn(len(monitorNames))]),
		AlertMessage: generateAlertMessage(alertStatus),
		AlertStatus:  alertStatus,
		MonitorID:    rand.Int63n(10000) + 1,
		MonitorName:  monitorNames[rand.Intn(len(monitorNames))],
		MonitorType:  "metric alert",
		Tags:         generateTags(),
		Timestamp:    now.Unix(),
		EventType:    "alert",
		Priority:     priorities[rand.Intn(len(priorities))],
		Hostname:     hostnames[rand.Intn(len(hostnames))],
		Service:      services[rand.Intn(len(services))],
		Scope:        generateScope(),
		LastUpdated:  now.Unix(),
		OrgID:        12345,
		OrgName:      "Demo Organization",
	}
}

// GenerateRUMSession creates a fake RUM session
func GenerateRUMSession() (rum.Visitor, rum.Session, []rum.RUMEvent) {
	visitorUUID := uuid.New().String()
	sessionID := uuid.New().String()

	startTime := time.Now().Add(-time.Duration(rand.Intn(3600)) * time.Second)
	pageViews := rand.Intn(10) + 1
	durationMs := int64(rand.Intn(300000) + 10000) // 10s - 5min

	userAgent := userAgents[rand.Intn(len(userAgents))]
	entryPage := pagePaths[rand.Intn(len(pagePaths))]
	referrer := referrers[rand.Intn(len(referrers))]

	visitor := rum.Visitor{
		UUID:         visitorUUID,
		FirstSeen:    startTime,
		LastSeen:     startTime.Add(time.Duration(durationMs) * time.Millisecond),
		SessionCount: 1,
		TotalViews:   pageViews,
		UserAgent:    userAgent,
		IPHash:       fmt.Sprintf("hash_%d", rand.Intn(1000000)),
	}

	session := rum.Session{
		VisitorUUID: visitorUUID,
		SessionID:   sessionID,
		StartTime:   startTime,
		PageViews:   pageViews,
		DurationMs:  durationMs,
		Referrer:    referrer,
		EntryPage:   entryPage,
		UserAgent:   userAgent,
	}

	// Generate events for the session
	var events []rum.RUMEvent
	currentTime := startTime

	for i := 0; i < pageViews; i++ {
		pageIdx := rand.Intn(len(pagePaths))
		eventType := "view"
		if rand.Float32() < 0.1 {
			eventType = "action"
		}
		if rand.Float32() < 0.05 {
			eventType = "error"
		}

		event := rum.RUMEvent{
			VisitorUUID: visitorUUID,
			SessionID:   sessionID,
			EventType:   eventType,
			Timestamp:   currentTime,
			PageURL:     "https://demo.example.com" + pagePaths[pageIdx],
			PageTitle:   pageTitles[pageIdx],
			Duration:    int64(rand.Intn(5000) + 500),
		}

		if eventType == "action" {
			event.ActionName = fmt.Sprintf("click_button_%d", rand.Intn(10))
			event.ActionType = "click"
		}
		if eventType == "error" {
			event.ErrorMsg = fmt.Sprintf("JavaScript error: undefined is not a function (line %d)", rand.Intn(1000))
		}

		events = append(events, event)
		currentTime = currentTime.Add(time.Duration(rand.Intn(30)+5) * time.Second)
	}

	return visitor, session, events
}

// GenerateMonitorAlert creates a fake monitor alert for demo purposes
func GenerateMonitorAlert() map[string]interface{} {
	alertStatus := alertStatuses[rand.Intn(len(alertStatuses))]

	return map[string]interface{}{
		"id":            rand.Int63n(10000) + 1,
		"name":          monitorNames[rand.Intn(len(monitorNames))],
		"type":          "metric alert",
		"status":        alertStatus,
		"overall_state": alertStatus,
		"tags":          generateTags(),
		"created":       time.Now().Add(-time.Duration(rand.Intn(30*24)) * time.Hour).Format(time.RFC3339),
		"modified":      time.Now().Add(-time.Duration(rand.Intn(24)) * time.Hour).Format(time.RFC3339),
		"message":       generateAlertMessage(alertStatus),
	}
}

// Helper functions

func generateAlertMessage(status string) string {
	switch status {
	case "Alert":
		return fmt.Sprintf("CPU usage exceeded 90%% threshold on %s. Current value: %.1f%%",
			hostnames[rand.Intn(len(hostnames))], 90.0+rand.Float64()*10)
	case "Warn":
		return fmt.Sprintf("CPU usage is approaching threshold on %s. Current value: %.1f%%",
			hostnames[rand.Intn(len(hostnames))], 75.0+rand.Float64()*15)
	case "OK":
		return fmt.Sprintf("CPU usage has returned to normal on %s. Current value: %.1f%%",
			hostnames[rand.Intn(len(hostnames))], 30.0+rand.Float64()*30)
	default:
		return fmt.Sprintf("No data received from %s in the last 5 minutes",
			hostnames[rand.Intn(len(hostnames))])
	}
}

func generateTags() []string {
	numTags := rand.Intn(5) + 2
	tags := make([]string, numTags)

	tags[0] = scopes[rand.Intn(3)]  // env tag
	tags[1] = scopes[3+rand.Intn(3)] // team tag

	for i := 2; i < numTags; i++ {
		tags[i] = scopes[rand.Intn(len(scopes))]
	}

	return tags
}

func generateScope() string {
	numScopes := rand.Intn(3) + 1
	scopeList := make([]string, numScopes)

	for i := 0; i < numScopes; i++ {
		scopeList[i] = scopes[rand.Intn(len(scopes))]
	}

	result := ""
	for i, s := range scopeList {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
