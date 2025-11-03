package orbi

import (
	"context"
	"time"
)

// Message represents a single message in the conversation
type Message struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Intent represents the parsed user intent
type Intent struct {
	Type           string                 `json:"type"`            // "query", "create", "update", "delete"
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
	Action       string                 `json:"action"`        // "create", "update", "delete", "query"
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

// Tool represents a callable function that an agent can use
type Tool interface {
	// Name returns the tool's identifier
	Name() string

	// Description returns a human-readable description
	Description() string

	// Execute performs the tool's operation
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResponse, error)

	// RequiresApproval returns true if execution needs user confirmation
	RequiresApproval() bool
}

// RetrievalTool interface for database queries
type RetrievalTool interface {
	Tool

	// GetEvents retrieves events in a time range
	GetEvents(ctx context.Context, startTime, endTime time.Time, filters map[string]interface{}) ([]map[string]interface{}, error)

	// GetAvailableSlots finds free time slots
	GetAvailableSlots(ctx context.Context, startTime, endTime time.Time, duration time.Duration) ([]map[string]interface{}, error)

	// GetEventDetails retrieves full details of an event
	GetEventDetails(ctx context.Context, eventID string) (map[string]interface{}, error)

	// QueryHistory retrieves past interactions
	QueryHistory(ctx context.Context, sessionID string, limit int) ([]Message, error)
}

// UpdateTool interface for transactional operations
type UpdateTool interface {
	Tool

	// CreateEvent creates a new calendar event
	CreateEvent(ctx context.Context, eventData map[string]interface{}) (map[string]interface{}, error)

	// UpdateEvent modifies an existing event
	UpdateEvent(ctx context.Context, eventID string, updates map[string]interface{}) (map[string]interface{}, error)

	// DeleteEvent removes an event
	DeleteEvent(ctx context.Context, eventID string) error

	// ApplyPendingApproval executes a pre-approved change
	ApplyPendingApproval(ctx context.Context, approval *PendingApproval) error
}

// ConfirmationValidator interface for safety checks
type ConfirmationValidator interface {
	// ValidateChange checks if a proposed change is valid
	ValidateChange(ctx context.Context, changeType string, oldState, newState map[string]interface{}) (bool, string, error)

	// GenerateHumanSummary creates a user-friendly description
	GenerateHumanSummary(ctx context.Context, intent *Intent, changes map[string]interface{}) (string, error)

	// CheckConflicts detects scheduling conflicts
	CheckConflicts(ctx context.Context, eventData map[string]interface{}) ([]map[string]interface{}, error)
}

// AgentMemory interface for conversation state management
type AgentMemory interface {
	// SaveMessage adds a message to history
	SaveMessage(ctx context.Context, sessionID string, msg Message) error

	// GetMessages retrieves conversation history
	GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error)

	// ClearSession removes a session
	ClearSession(ctx context.Context, sessionID string) error

	// SaveIntent caches an identified intent
	SaveIntent(ctx context.Context, sessionID string, intent *Intent) error

	// GetIntent retrieves a cached intent
	GetIntent(ctx context.Context, sessionID string, key string) (*Intent, error)
}

// AuditTrail interface for logging
type AuditTrail interface {
	// LogAction records an action
	LogAction(ctx context.Context, log AuditLog) error

	// GetAuditLog retrieves audit logs
	GetAuditLog(ctx context.Context, sessionID string, limit int) ([]AuditLog, error)

	// VerifyAction checks if an action was approved
	VerifyAction(ctx context.Context, logID string) (bool, error)
}

// IntentClassifier defines the contract for intent parsing
type IntentClassifier interface {
	// Classify analyzes user input and returns an Intent
	Classify(ctx context.Context, userInput string, conversationHistory []Message) (*Intent, error)
}

// ResponseGenerator defines the contract for generating user responses
type ResponseGenerator interface {
	// Generate creates a natural language response
	Generate(ctx context.Context, intent *Intent, toolResponses map[string]*ToolResponse) (string, error)

	// GenerateConfirmationPrompt creates a confirmation message
	GenerateConfirmationPrompt(ctx context.Context, approval *PendingApproval) (string, error)
}

// PlannerAgent is the primary orchestrator
type PlannerAgent interface {
	SubAgent

	// Process handles the full flow from user input to response
	Process(ctx context.Context, userInput string, context *ConversationContext) (string, error)

	// RegisterSubAgent adds a sub-agent
	RegisterSubAgent(agent SubAgent) error

	// RegisterTool adds a tool
	RegisterTool(tool Tool) error
}
