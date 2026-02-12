package github

import "time"

// IssueEvent represents a GitHub Issues webhook event payload.
// See: https://docs.github.com/en/webhooks/webhook-events-and-payloads#issues
type IssueEvent struct {
	Action string `json:"action"` // opened, edited, deleted, pinned, unpinned, closed, reopened, assigned, unassigned, labeled, unlabeled, locked, unlocked, transferred, milestoned, demilestoned
	Issue  Issue  `json:"issue"`
	Sender User   `json:"sender"`
	Repo   Repo   `json:"repository"`
}

// Issue represents a GitHub issue.
type Issue struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"` // open, closed
	HTMLURL   string     `json:"html_url"`
	URL       string     `json:"url"`
	User      User       `json:"user"`
	Labels    []Label    `json:"labels"`
	Assignees []User     `json:"assignees"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

// User represents a GitHub user (sender, assignee, etc).
type User struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

// Repo represents a GitHub repository.
type Repo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	Private  bool   `json:"private"`
}

// Label represents a GitHub issue label.
type Label struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// StoredIssueEvent is a GitHub issue event persisted in PostgreSQL.
type StoredIssueEvent struct {
	ID          int64      `json:"id"`
	DeliveryID  string     `json:"delivery_id,omitempty"`
	Action      string     `json:"action"`
	IssueID     int64      `json:"issue_id"`
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	State       string     `json:"state"`
	HTMLURL     string     `json:"html_url"`
	SenderLogin string     `json:"sender_login"`
	RepoName    string     `json:"repo_name"`
	Labels      []string   `json:"labels"`
	ReceivedAt  time.Time  `json:"received_at"`
	RawPayload  []byte     `json:"raw_payload,omitempty"`
	AgentStatus string     `json:"agent_status"`
	AgentResult string     `json:"agent_result,omitempty"`
	BranchName  string     `json:"branch_name,omitempty"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}

// IssueEventListResponse is the paginated response for listing stored events.
type IssueEventListResponse struct {
	Events     []StoredIssueEvent `json:"events"`
	TotalCount int                `json:"total_count"`
	Page       int                `json:"page"`
	PerPage    int                `json:"per_page"`
}

// AgentStatus tracks the processing state for a GitHub issue.
type AgentStatus string

const (
	AgentStatusPending    AgentStatus = "pending"
	AgentStatusProcessing AgentStatus = "processing"
	AgentStatusCompleted  AgentStatus = "completed"
	AgentStatusFailed     AgentStatus = "failed"
	AgentStatusSkipped    AgentStatus = "skipped"
)

// PastIssue represents a previously processed GitHub issue for dedup cross-referencing.
type PastIssue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Summary     string `json:"summary,omitempty"`
	BranchName  string `json:"branch_name,omitempty"`
	AgentStatus string `json:"agent_status"`
}

// AgentProcessRequest is the payload sent to the Claude agent sidecar.
type AgentProcessRequest struct {
	IssueNumber int         `json:"issue_number"`
	IssueTitle  string      `json:"issue_title"`
	IssueBody   string      `json:"issue_body"`
	IssueURL    string      `json:"issue_url"`
	RepoName    string      `json:"repo_name"`
	SenderLogin string      `json:"sender_login"`
	EventID     int64       `json:"event_id"`
	PastIssues  []PastIssue `json:"past_issues,omitempty"`
}

// AgentProcessResponse is the response from the Claude agent sidecar.
type AgentProcessResponse struct {
	Success       bool     `json:"success"`
	Duplicate     bool     `json:"duplicate,omitempty"`
	DuplicateOf   int      `json:"duplicate_of,omitempty"`
	BranchName    string   `json:"branch_name,omitempty"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	CommentURL    string   `json:"comment_url,omitempty"`
	Error         string   `json:"error,omitempty"`
}
