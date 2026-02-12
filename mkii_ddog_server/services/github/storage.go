package github

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/lib/pq"
)

// Storage handles database operations for GitHub issue events.
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new GitHub storage instance.
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// InitTables creates the necessary database tables.
func (s *Storage) InitTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS github_issue_events (
		id SERIAL PRIMARY KEY,
		delivery_id VARCHAR(255) UNIQUE,
		action VARCHAR(50) NOT NULL,
		issue_id BIGINT NOT NULL,
		number INT NOT NULL,
		title TEXT,
		body TEXT,
		state VARCHAR(20),
		html_url TEXT,
		sender_login VARCHAR(255),
		repo_name VARCHAR(255),
		labels TEXT[],
		received_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		raw_payload JSONB
	);

	CREATE INDEX IF NOT EXISTS idx_github_issue_events_issue_id ON github_issue_events(issue_id);
	CREATE INDEX IF NOT EXISTS idx_github_issue_events_action ON github_issue_events(action);
	CREATE INDEX IF NOT EXISTS idx_github_issue_events_repo ON github_issue_events(repo_name);
	CREATE INDEX IF NOT EXISTS idx_github_issue_events_received_at ON github_issue_events(received_at);
	`

	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}

	// Add delivery_id column if it doesn't exist (for existing tables)
	alterQuery := `
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'github_issue_events' AND column_name = 'delivery_id') THEN
			ALTER TABLE github_issue_events ADD COLUMN delivery_id VARCHAR(255) UNIQUE;
		END IF;
	END $$;
	`
	_, err = s.db.Exec(alterQuery)
	if err != nil {
		return err
	}

	// Add agent processing columns if they don't exist
	agentQuery := `
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'github_issue_events' AND column_name = 'agent_status') THEN
			ALTER TABLE github_issue_events ADD COLUMN agent_status VARCHAR(20) DEFAULT 'pending';
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'github_issue_events' AND column_name = 'agent_result') THEN
			ALTER TABLE github_issue_events ADD COLUMN agent_result JSONB;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'github_issue_events' AND column_name = 'branch_name') THEN
			ALTER TABLE github_issue_events ADD COLUMN branch_name VARCHAR(255);
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'github_issue_events' AND column_name = 'processed_at') THEN
			ALTER TABLE github_issue_events ADD COLUMN processed_at TIMESTAMP WITH TIME ZONE;
		END IF;
	END $$;
	CREATE INDEX IF NOT EXISTS idx_github_issue_events_agent_status ON github_issue_events(agent_status);
	`
	_, err = s.db.Exec(agentQuery)
	return err
}

// StoreEvent persists a GitHub issue event with deduplication via delivery_id.
func (s *Storage) StoreEvent(evt IssueEvent, deliveryID string) (*StoredIssueEvent, error) {
	rawPayload, err := json.Marshal(evt)
	if err != nil {
		log.Printf("[GITHUB] Warning: failed to marshal raw payload: %v", err)
	}

	labels := make([]string, len(evt.Issue.Labels))
	for i, l := range evt.Issue.Labels {
		labels[i] = l.Name
	}

	query := `
	INSERT INTO github_issue_events (
		delivery_id, action, issue_id, number, title, body, state,
		html_url, sender_login, repo_name, labels, raw_payload
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	RETURNING id, received_at`

	stored := &StoredIssueEvent{
		DeliveryID:  deliveryID,
		Action:      evt.Action,
		IssueID:     evt.Issue.ID,
		Number:      evt.Issue.Number,
		Title:       evt.Issue.Title,
		Body:        evt.Issue.Body,
		State:       evt.Issue.State,
		HTMLURL:     evt.Issue.HTMLURL,
		SenderLogin: evt.Sender.Login,
		RepoName:    evt.Repo.FullName,
		Labels:      labels,
		RawPayload:  rawPayload,
	}

	err = s.db.QueryRow(
		query,
		deliveryID, stored.Action, stored.IssueID, stored.Number, stored.Title,
		stored.Body, stored.State, stored.HTMLURL, stored.SenderLogin,
		stored.RepoName, pq.Array(labels), rawPayload,
	).Scan(&stored.ID, &stored.ReceivedAt)

	if err != nil {
		return nil, err
	}

	return stored, nil
}

// GetRecentEvents retrieves recent GitHub issue events with pagination.
func (s *Storage) GetRecentEvents(limit, offset int) ([]StoredIssueEvent, int, error) {
	var totalCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM github_issue_events`).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	query := `
	SELECT id, delivery_id, action, issue_id, number, title, body, state,
		html_url, sender_login, repo_name, labels, received_at
	FROM github_issue_events
	ORDER BY received_at DESC
	LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []StoredIssueEvent
	for rows.Next() {
		var evt StoredIssueEvent
		var labels pq.StringArray
		var body sql.NullString
		var deliveryID sql.NullString

		err := rows.Scan(
			&evt.ID, &deliveryID, &evt.Action, &evt.IssueID, &evt.Number,
			&evt.Title, &body, &evt.State, &evt.HTMLURL,
			&evt.SenderLogin, &evt.RepoName, &labels, &evt.ReceivedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		evt.Labels = labels
		if body.Valid {
			evt.Body = body.String
		}
		if deliveryID.Valid {
			evt.DeliveryID = deliveryID.String
		}

		events = append(events, evt)
	}

	return events, totalCount, nil
}

// GetEventByID retrieves a single GitHub issue event.
func (s *Storage) GetEventByID(id int64) (*StoredIssueEvent, error) {
	query := `
	SELECT id, delivery_id, action, issue_id, number, title, body, state,
		html_url, sender_login, repo_name, labels, received_at, raw_payload
	FROM github_issue_events WHERE id = $1`

	var evt StoredIssueEvent
	var labels pq.StringArray
	var body sql.NullString
	var deliveryID sql.NullString
	var rawPayload []byte

	err := s.db.QueryRow(query, id).Scan(
		&evt.ID, &deliveryID, &evt.Action, &evt.IssueID, &evt.Number,
		&evt.Title, &body, &evt.State, &evt.HTMLURL,
		&evt.SenderLogin, &evt.RepoName, &labels, &evt.ReceivedAt, &rawPayload,
	)
	if err != nil {
		return nil, err
	}

	evt.Labels = labels
	evt.RawPayload = rawPayload
	if body.Valid {
		evt.Body = body.String
	}
	if deliveryID.Valid {
		evt.DeliveryID = deliveryID.String
	}

	return &evt, nil
}

// GetStats returns aggregate statistics for GitHub issue events.
func (s *Storage) GetStats() (map[string]any, error) {
	stats := make(map[string]any)

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM github_issue_events`).Scan(&total); err != nil {
		return nil, err
	}
	stats["total_events"] = total

	actionCounts := make(map[string]int)
	rows, err := s.db.Query(`SELECT action, COUNT(*) FROM github_issue_events GROUP BY action`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var action string
		var count int
		if err := rows.Scan(&action, &count); err != nil {
			return nil, err
		}
		actionCounts[action] = count
	}
	stats["by_action"] = actionCounts

	repoCounts := make(map[string]int)
	rows2, err := s.db.Query(`SELECT repo_name, COUNT(*) FROM github_issue_events GROUP BY repo_name`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var repo string
		var count int
		if err := rows2.Scan(&repo, &count); err != nil {
			return nil, err
		}
		repoCounts[repo] = count
	}
	stats["by_repo"] = repoCounts

	return stats, nil
}

// HasBeenProcessed checks if a GitHub issue has already been processed by the agent.
func (s *Storage) HasBeenProcessed(issueID int64, repoName string) (bool, string, error) {
	var branchName sql.NullString
	query := `SELECT branch_name FROM github_issue_events
	          WHERE issue_id = $1 AND repo_name = $2 AND agent_status = 'completed'
	          LIMIT 1`
	err := s.db.QueryRow(query, issueID, repoName).Scan(&branchName)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, branchName.String, nil
}

// UpdateAgentStatus updates the agent processing status for a stored event.
func (s *Storage) UpdateAgentStatus(eventID int64, status, result, branchName string) error {
	// Handle empty result string - store as NULL instead of invalid JSON
	var resultVal interface{}
	if result == "" {
		resultVal = nil
	} else {
		resultVal = result
	}

	query := `UPDATE github_issue_events
	          SET agent_status = $1, agent_result = $2, branch_name = $3, processed_at = NOW()
	          WHERE id = $4`
	_, err := s.db.Exec(query, status, resultVal, branchName, eventID)
	return err
}

// GetCompletedIssues retrieves past completed/failed/skipped issues for a given repo.
func (s *Storage) GetCompletedIssues(repoName string) ([]PastIssue, error) {
	query := `SELECT number, title, agent_result, branch_name, agent_status
	          FROM github_issue_events
	          WHERE repo_name = $1 AND agent_status IN ('completed', 'failed', 'skipped')
	          ORDER BY received_at DESC
	          LIMIT 50`
	rows, err := s.db.Query(query, repoName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var issues []PastIssue
	for rows.Next() {
		var p PastIssue
		var agentResult sql.NullString
		var branchName sql.NullString
		err := rows.Scan(&p.Number, &p.Title, &agentResult, &branchName, &p.AgentStatus)
		if err != nil {
			return nil, err
		}
		if branchName.Valid {
			p.BranchName = branchName.String
		}
		// Extract summary from agent_result JSON if available
		if agentResult.Valid && agentResult.String != "" {
			var result map[string]interface{}
			if json.Unmarshal([]byte(agentResult.String), &result) == nil {
				if s, ok := result["summary"].(string); ok {
					p.Summary = s
				}
			}
		}
		issues = append(issues, p)
	}
	return issues, nil
}
