package domain

import "time"

// ClickEvent represents a single search result click to be tracked.
type ClickEvent struct {
	QueryID         string    `json:"query_id"`
	ResultID        string    `json:"result_id"`
	Position        int       `json:"position"`
	Page            int       `json:"page"`
	DestinationHash string    `json:"destination_hash"`
	SessionID       string    `json:"session_id,omitempty"`
	UserAgentHash   string    `json:"user_agent_hash,omitempty"`
	GeneratedAt     time.Time `json:"generated_at"`
	ClickedAt       time.Time `json:"clicked_at"`
}
