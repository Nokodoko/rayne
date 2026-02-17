package webhooks

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

// Notifier sends desktop notifications for webhook processing events.
// Used by the orchestrator to notify when notebooks are created or agent analysis completes.
type Notifier struct {
	serverURLs []string
	client     *http.Client
}

// NewNotifier creates a notifier using the NOTIFY_SERVER_URLS/NOTIFY_SERVER_URL env vars.
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

// NotifyNotebookCreated sends a desktop notification when a Datadog notebook is created.
func (n *Notifier) NotifyNotebookCreated(monitorName string, agentRole string, notebookURL string) {
	title := fmt.Sprintf("📓 Notebook Created [%s]", agentRole)
	message := monitorName
	if notebookURL != "" {
		message = fmt.Sprintf("%s\n%s", monitorName, notebookURL)
	}

	n.send(title, message, "normal")
}

// send posts a notification payload to all configured servers.
func (n *Notifier) send(title, message, urgency string) {
	payload := map[string]string{
		"title":   title,
		"message": message,
		"urgency": urgency,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[WEBHOOK-NOTIFY] Failed to marshal payload: %v", err)
		return
	}

	for _, serverURL := range n.serverURLs {
		resp, err := n.client.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[WEBHOOK-NOTIFY] Error sending to %s: %v", serverURL, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("[WEBHOOK-NOTIFY] Server %s returned HTTP %d", serverURL, resp.StatusCode)
			continue
		}
		log.Printf("[WEBHOOK-NOTIFY] Notification sent to %s", serverURL)
	}
}
