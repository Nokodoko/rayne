package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// notifyColorGray is the border color for GitHub issue notifications
	notifyColorGray = "#808080"
	maxTitleLen     = 200
	maxMessageLen   = 500
)

// truncate truncates a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Notifier sends desktop notifications for GitHub webhook events.
type Notifier struct {
	serverURLs []string
	client     *http.Client
}

// NewNotifier creates a notifier using the same NOTIFY_SERVER_URLS/NOTIFY_SERVER_URL
// env vars as the Datadog DesktopNotifyProcessor.
func NewNotifier() *Notifier {
	var urls []string

	if urlList := os.Getenv("NOTIFY_SERVER_URLS"); urlList != "" {
		for _, u := range strings.Split(urlList, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				urls = append(urls, u)
			}
		}
	}

	if len(urls) == 0 {
		url := os.Getenv("NOTIFY_SERVER_URL")
		if url == "" {
			url = "http://host.minikube.internal:9999"
		}
		urls = append(urls, url)
	}

	return &Notifier{
		serverURLs: urls,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

// NotifyIssueEvent sends a desktop notification for a GitHub issue event.
func (n *Notifier) NotifyIssueEvent(evt IssueEvent) {
	title := truncate(fmt.Sprintf("GitHub: %s #%d", evt.Action, evt.Issue.Number), maxTitleLen)
	message := truncate(fmt.Sprintf("[%s] %s", evt.Repo.FullName, evt.Issue.Title), maxMessageLen)

	payload := map[string]string{
		"title":   title,
		"message": message,
		"urgency": "normal",
		"frcolor": notifyColorGray,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[GITHUB-NOTIFY] Failed to marshal payload: %v", err)
		return
	}

	for _, serverURL := range n.serverURLs {
		resp, err := n.client.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[GITHUB-NOTIFY] Error sending to %s: %v", serverURL, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("[GITHUB-NOTIFY] Server %s returned HTTP %d", serverURL, resp.StatusCode)
			continue
		}
		log.Printf("[GITHUB-NOTIFY] Notification sent to %s", serverURL)
	}
}

// NotifyAgentResult sends a desktop notification when agent processing completes.
func (n *Notifier) NotifyAgentResult(issueNumber int, title string, repoName string, success bool) {
	status := "completed"
	urgency := "normal"
	if !success {
		status = "FAILED"
		urgency = "critical"
	}

	notifTitle := truncate(fmt.Sprintf("Agent %s: #%d", status, issueNumber), maxTitleLen)
	message := truncate(fmt.Sprintf("[%s] %s", repoName, title), maxMessageLen)

	payload := map[string]string{
		"title":   notifTitle,
		"message": message,
		"urgency": urgency,
		"frcolor": notifyColorGray,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[GITHUB-NOTIFY] Failed to marshal agent result payload: %v", err)
		return
	}

	for _, serverURL := range n.serverURLs {
		resp, err := n.client.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[GITHUB-NOTIFY] Error sending to %s: %v", serverURL, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("[GITHUB-NOTIFY] Server %s returned HTTP %d", serverURL, resp.StatusCode)
			continue
		}
		log.Printf("[GITHUB-NOTIFY] Agent result notification sent to %s", serverURL)
	}
}
