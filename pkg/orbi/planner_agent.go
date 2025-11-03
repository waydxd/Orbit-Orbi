package orbi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PlannerChatbotAgent is the primary orchestrator for the conversation flow
type PlannerChatbotAgent struct {
	name              string
	llm               LLMInterface
	intentClassifier  IntentClassifier
	responseGenerator ResponseGenerator
	memory            AgentMemory
	auditTrail        AuditTrail

	// Sub-agents
	retrievalAgent   SubAgent
	updateAgent      SubAgent
	confirmationAgent SubAgent

	// Tools
	tools map[string]Tool

	// Configuration
	maxIterations int
	mu            sync.RWMutex
}

// NewPlannerChatbotAgent creates a new Planner Chatbot Agent
func NewPlannerChatbotAgent(
	llm LLMInterface,
	classifier IntentClassifier,
	generator ResponseGenerator,
	memory AgentMemory,
	auditTrail AuditTrail,
) *PlannerChatbotAgent {
	return &PlannerChatbotAgent{
		name:              "planner-chatbot-agent",
		llm:               llm,
		intentClassifier:  classifier,
		responseGenerator: generator,
		memory:            memory,
		auditTrail:        auditTrail,
		tools:             make(map[string]Tool),
		maxIterations:     5,
	}
}

// Name returns the agent's identifier
func (a *PlannerChatbotAgent) Name() string {
	return a.name
}

// ShouldHandle always returns true as the planner handles all requests
func (a *PlannerChatbotAgent) ShouldHandle(intent *Intent) bool {
	return true
}

// Process handles the complete flow from user input to response
func (a *PlannerChatbotAgent) Process(
	ctx context.Context,
	userInput string,
	context *ConversationContext,
) (string, error) {
	// Step 1: Save user message to memory
	userMsg := Message{
		Role:      "user",
		Content:   userInput,
		Timestamp: time.Now(),
	}

	if a.memory != nil {
		if err := a.memory.SaveMessage(ctx, context.SessionID, userMsg); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to save message to memory: %v\n", err)
		}
	}

	context.MessageHistory = append(context.MessageHistory, userMsg)

	// Step 2: Classify the user intent
	intent, err := a.intentClassifier.Classify(ctx, userInput, context.MessageHistory)
	if err != nil {
		return "", fmt.Errorf("failed to classify intent: %w", err)
	}

	// Cache the intent
	context.IntentCache["last"] = *intent

	// Step 3: Route to appropriate sub-agent
	var response string

	switch intent.Type {
	case "query", "read":
		if a.retrievalAgent != nil && a.retrievalAgent.ShouldHandle(intent) {
			response, err = a.retrievalAgent.Handle(ctx, intent, context)
			if err != nil {
				return "", fmt.Errorf("retrieval agent failed: %w", err)
			}
		}

	case "write", "modify":
		if a.updateAgent != nil && a.updateAgent.ShouldHandle(intent) {
			response, err = a.updateAgent.Handle(ctx, intent, context)
			if err != nil {
				return "", fmt.Errorf("update agent failed: %w", err)
			}
		}

	case "confirmation":
		if a.confirmationAgent != nil && a.confirmationAgent.ShouldHandle(intent) {
			response, err = a.confirmationAgent.Handle(ctx, intent, context)
			if err != nil {
				return "", fmt.Errorf("confirmation agent failed: %w", err)
			}
		}

	default:
		// Try to handle with general conversation
		response, err = a.handleGeneralConversation(ctx, userInput, intent, context)
		if err != nil {
			return "", fmt.Errorf("general conversation failed: %w", err)
		}
	}

	// Step 4: Generate final response if not already generated
	if response == "" && a.responseGenerator != nil {
		response, err = a.responseGenerator.Generate(ctx, intent, nil)
		if err != nil {
			// Fallback to simple response
			response = "I'm not sure how to respond to that. Could you provide more details?"
		}
	}

	// Step 5: Save assistant response to memory
	assistantMsg := Message{
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	}

	if a.memory != nil {
		if err := a.memory.SaveMessage(ctx, context.SessionID, assistantMsg); err != nil {
			fmt.Printf("Warning: failed to save response to memory: %v\n", err)
		}
	}

	context.MessageHistory = append(context.MessageHistory, assistantMsg)

	// Step 6: Update last activity time
	context.LastActivityAt = time.Now()

	// Step 7: Log the interaction
	if a.auditTrail != nil {
		a.auditTrail.LogAction(ctx, AuditLog{
			ID:        uuid.New().String(),
			SessionID: context.SessionID,
			UserID:    context.UserID,
			Action:    intent.Type,
			Status:    "executed",
			Metadata: map[string]interface{}{
				"intent":  intent,
				"message": userInput,
			},
			Timestamp: time.Now(),
		})
	}

	return response, nil
}

// Handle implements the SubAgent interface
func (a *PlannerChatbotAgent) Handle(
	ctx context.Context,
	intent *Intent,
	context *ConversationContext,
) (string, error) {
	// Delegate to Process
	if context == nil {
		context = &ConversationContext{
			SessionID:        uuid.New().String(),
			MessageHistory:   []Message{},
			IntentCache:      make(map[string]Intent),
			PendingApprovals: make(map[string]*PendingApproval),
			AuditLogs:        []AuditLog{},
			CreatedAt:        time.Now(),
		}
	}

	// Simple implementation: reconstruct user input from intent and handle
	// In real usage, this would be called via Process()
	return "", fmt.Errorf("use Process() method instead")
}

// RegisterSubAgent adds a sub-agent
func (a *PlannerChatbotAgent) RegisterSubAgent(agent SubAgent) error {
	if agent == nil {
		return fmt.Errorf("sub-agent cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	switch agent.Name() {
	case "retrieval-agent":
		a.retrievalAgent = agent
	case "update-agent":
		a.updateAgent = agent
	case "confirmation-agent":
		a.confirmationAgent = agent
	default:
		return fmt.Errorf("unknown sub-agent type: %s", agent.Name())
	}

	return nil
}

// RegisterTool adds a tool
func (a *PlannerChatbotAgent) RegisterTool(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.tools[tool.Name()] = tool
	return nil
}

// GetTool retrieves a registered tool
func (a *PlannerChatbotAgent) GetTool(name string) (Tool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tool, ok := a.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool, nil
}

// handleGeneralConversation handles non-specific conversation
func (a *PlannerChatbotAgent) handleGeneralConversation(
	ctx context.Context,
	userInput string,
	intent *Intent,
	context *ConversationContext,
) (string, error) {
	if a.llm == nil {
		return "I'm Orbi, your smart calendar assistant. How can I help you with your calendar?", nil
	}

	// Build conversation context for LLM
	var conversationText string
	for _, msg := range context.MessageHistory[max(0, len(context.MessageHistory)-6):] { // Last 3 exchanges
		role := "You"
		if msg.Role == "assistant" {
			role = "Orbi"
		}
		conversationText += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	prompt := fmt.Sprintf(`You are Orbi, a helpful smart calendar assistant. 
You help users manage their calendar through natural conversation in both English and Chinese.
You can help with:
- Creating, updating, or deleting events
- Finding available time slots
- Checking calendar events
- Rescheduling meetings
- Answering questions about schedule

Recent conversation:
%s

User: %s

Respond naturally and helpfully. If the user wants to make a change, ask for confirmation.
Keep responses concise and friendly.`,
		conversationText, userInput)

	response, err := a.llm.Generate(ctx, prompt)
	if err != nil {
		return "I encountered an issue. Could you please try again?", nil
	}

	return response, nil
}

// CreateSession creates a new conversation session
func (a *PlannerChatbotAgent) CreateSession(userID string) *ConversationContext {
	return &ConversationContext{
		SessionID:        uuid.New().String(),
		UserID:           userID,
		MessageHistory:   []Message{},
		IntentCache:      make(map[string]Intent),
		PendingApprovals: make(map[string]*PendingApproval),
		AuditLogs:        []AuditLog{},
		CreatedAt:        time.Now(),
		LastActivityAt:   time.Now(),
		ContextWindow:    20, // Keep last 20 messages in context
	}
}

// GetSessionHistory retrieves session history
func (a *PlannerChatbotAgent) GetSessionHistory(
	ctx context.Context,
	sessionID string,
	limit int,
) ([]Message, error) {
	if a.memory == nil {
		return []Message{}, fmt.Errorf("memory not configured")
	}

	return a.memory.GetMessages(ctx, sessionID, limit)
}

// ClearSession clears a session
func (a *PlannerChatbotAgent) ClearSession(ctx context.Context, sessionID string) error {
	if a.memory == nil {
		return fmt.Errorf("memory not configured")
	}

	return a.memory.ClearSession(ctx, sessionID)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ChatState represents the current state of a chat
type ChatState struct {
	SessionID        string
	UserID           string
	CurrentIntent    *Intent
	PendingApproval  *PendingApproval
	LastMessageTime  time.Time
	MessageCount     int
	ConversationData map[string]interface{}
}

// GetChatState returns the current state of a chat session
func (a *PlannerChatbotAgent) GetChatState(context *ConversationContext) *ChatState {
	var currentIntent *Intent
	if len(context.MessageHistory) > 0 {
		if intent, ok := context.IntentCache["last"]; ok {
			currentIntent = &intent
		}
	}

	var pendingApproval *PendingApproval
	if len(context.PendingApprovals) > 0 {
		// Get the most recent pending approval
		for _, pa := range context.PendingApprovals {
			if pa.ExpiresAt.After(time.Now()) {
				pendingApproval = pa
				break
			}
		}
	}

	return &ChatState{
		SessionID:       context.SessionID,
		UserID:          context.UserID,
		CurrentIntent:   currentIntent,
		PendingApproval: pendingApproval,
		LastMessageTime: context.LastActivityAt,
		MessageCount:    len(context.MessageHistory),
		ConversationData: make(map[string]interface{}),
	}
}
