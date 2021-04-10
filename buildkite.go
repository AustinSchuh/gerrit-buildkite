package main

// Structure definitions for Buildkite webhook events.

type BuildkiteChange struct {
	ID     string `json:"id,omitempty"`
	Number int    `json:"number,omitempty"`
	URL    string `json:"url,omitempty"`
}

type Build struct {
	ID           string           `json:"id,omitempty"`
	GraphqlId    string           `json:"graphql_id,omitempty"`
	URL          string           `json:"url,omitempty"`
	WebURL       string           `json:"web_url,omitempty"`
	Number       int              `json:"number,omitempty"`
	State        string           `json:"state,omitempty"`
	Blocked      bool             `json:"blocked,omitempty"`
	BlockedState string           `json:"blocked_state,omitempty"`
	Message      string           `json:"message,omitempty"`
	Commit       string           `json:"commit"`
	Branch       string           `json:"branch"`
	Source       string           `json:"source,omitempty"`
	CreatedAt    string           `json:"created_at,omitempty"`
	ScheduledAt  string           `json:"scheduled_at,omitempty"`
	StartedAt    string           `json:"started_at,omitempty"`
	FinishedAt   string           `json:"finished_at,omitempty"`
	RebuiltFrom  *BuildkiteChange `json:"rebuilt_from,omitempty"`
}

type BuildkiteWebhook struct {
	Event string `json:"event"`
	Build Build  `json:"build"`
}
