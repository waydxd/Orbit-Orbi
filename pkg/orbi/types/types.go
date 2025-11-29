package types

import (
	"context"
	"time"
)

// Message represents a single message in the conversation
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Intent represents the parsed user intent
type Intent struct {
	Type           string                 `json:"type"` // "query", "create", "update", "delete"
	Action         string                 `json:"action"`          // specific action
	Confidence     float64                `json:"confidence"`      // 0.0-1.0
	Parameters     map[string]interface{} `json:"parameters"`      // parsed parameters
	RequiresApproval bool                 `json:"requires_approval"` // if destructive
}

// ToolResponse represents the result of a tool execution
type ToolResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error,omitempty"`
}

// PendingApproval represents a change awaiting user confirmation
type PendingApproval struct {
	ID                string                 `json:"id"`
	ChangeType        string                 `json:"change_type"` // "create", "update", "delete"
	ResourceType      string                 `json:"resource_type"` // "event", "attendee"
	ResourceID        string                 `json:"resource_id,omitempty"`
	OldState          map[string]interface{} `json:"old_state"`
	NewState          map[string]interface{} `json:"new_state"`
	Summary           string                 `json:"summary"` // human-readable description
	ExpiresAt         time.Time              `json:"expires_at"`
	RequiredApprovals int                    `json:"required_approvals"`
	Approvals         []string               `json:"approvals"` // user IDs who approved
}

// AuditLog represents a logged action
type AuditLog struct {
	ID           string                 `json:"id"`
	SessionID    string                 `json:"session_id"`
	UserID       string                 `json:"user_id"`
	Action       string                 `json:"action"` // "create", "update", "delete", "query"
	ResourceType string                 `json:"resource_type"` // "event", "attendee"
	ResourceID   string                 `json:"resource_id,omitempty"`
	OldValue     string                 `json:"old_value,omitempty"`
	NewValue     string                 `json:"new_value,omitempty"`
	Status       string                 `json:"status"`       // "pending", "approved", "rejected", "executed"
	Error        string                 `json:"error,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// ConversationContext holds the state of a conversation
type ConversationContext struct {
	SessionID         string
	UserID            string
	MessageHistory    []Message
	IntentCache       map[string]Intent
	PendingApprovals  map[string]*PendingApproval
	AuditLogs         []AuditLog
	CreatedAt         time.Time
	LastActivityAt    time.Time
	ContextWindow     int // max messages to keep in context
}

// SubAgent represents a specialized agent that handles specific responsibilities
type SubAgent interface {
	// Name returns the agent's identifier
	Name() string

	// Handle processes a request and returns a response
	Handle(ctx context.Context, intent *Intent, context *ConversationContext) (string, error)

	// ShouldHandle returns true if this agent should handle the given intent
	ShouldHandle(intent *Intent) bool
}

// PlannerAgent is the primary orchestrator
type PlannerAgent interface {
	SubAgent

	// Process handles the full flow from user input to response
	Process(ctx context.Context, userInput string, context *ConversationContext) (string, error)

	// RegisterSubAgent adds a sub-agent
	RegisterSubAgent(agent SubAgent) error
}
