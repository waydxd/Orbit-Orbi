package orbi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// InMemoryAgentMemory is a simple in-memory implementation of AgentMemory
type InMemoryAgentMemory struct {
	sessions map[string]*MemorySession
}

// MemorySession holds data for a single session
type MemorySession struct {
	Messages []Message
	Intents  map[string]*Intent
}

// NewInMemoryAgentMemory creates a new in-memory memory store
func NewInMemoryAgentMemory() *InMemoryAgentMemory {
	return &InMemoryAgentMemory{
		sessions: make(map[string]*MemorySession),
	}
}

// SaveMessage adds a message to history
func (m *InMemoryAgentMemory) SaveMessage(ctx context.Context, sessionID string, msg Message) error {
	session, ok := m.sessions[sessionID]
	if !ok {
		session = &MemorySession{
			Messages: []Message{},
			Intents:  make(map[string]*Intent),
		}
		m.sessions[sessionID] = session
	}

	session.Messages = append(session.Messages, msg)
	return nil
}

// GetMessages retrieves conversation history
func (m *InMemoryAgentMemory) GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	session, ok := m.sessions[sessionID]
	if !ok {
		return []Message{}, nil
	}

	if limit <= 0 || limit > len(session.Messages) {
		limit = len(session.Messages)
	}

	return session.Messages[len(session.Messages)-limit:], nil
}

// ClearSession removes a session
func (m *InMemoryAgentMemory) ClearSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

// SaveIntent caches an identified intent
func (m *InMemoryAgentMemory) SaveIntent(ctx context.Context, sessionID string, intent *Intent) error {
	session, ok := m.sessions[sessionID]
	if !ok {
		session = &MemorySession{
			Messages: []Message{},
			Intents:  make(map[string]*Intent),
		}
		m.sessions[sessionID] = session
	}

	session.Intents["last"] = intent
	return nil
}

// GetIntent retrieves a cached intent
func (m *InMemoryAgentMemory) GetIntent(ctx context.Context, sessionID string, key string) (*Intent, error) {
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	intent, ok := session.Intents[key]
	if !ok {
		return nil, fmt.Errorf("intent not found")
	}

	return intent, nil
}

// SimpleIntentClassifier is a basic rule-based intent classifier
type SimpleIntentClassifier struct {
	llm LLMInterface
}

// NewSimpleIntentClassifier creates a new intent classifier
func NewSimpleIntentClassifier(llm LLMInterface) *SimpleIntentClassifier {
	return &SimpleIntentClassifier{
		llm: llm,
	}
}

// Classify analyzes user input and returns an Intent
func (c *SimpleIntentClassifier) Classify(
	ctx context.Context,
	userInput string,
	conversationHistory []Message,
) (*Intent, error) {
	// Rule-based classification for common patterns
	intent := c.classifyByRules(userInput)
	if intent.Confidence > 0.7 {
		return intent, nil
	}

	// Fall back to LLM if confidence is low
	if c.llm != nil {
		return c.classifyWithLLM(ctx, userInput, conversationHistory)
	}

	return intent, nil
}

// classifyByRules uses pattern matching for common intents
func (c *SimpleIntentClassifier) classifyByRules(userInput string) *Intent {
	lower := strings.ToLower(userInput)

	// Query intents
	queryKeywords := []string{"when", "what", "show", "list", "get", "find", "search", "check", "有", "什麼", "幾", "幾時"}
	for _, kw := range queryKeywords {
		if strings.Contains(lower, kw) {
			return c.buildIntent("query", "search", 0.8, map[string]interface{}{
				"query": userInput,
			})
		}
	}

	// Create intents
	createKeywords := []string{"create", "new", "add", "schedule", "book", "建立", "新增", "安排", "預定"}
	for _, kw := range createKeywords {
		if strings.Contains(lower, kw) {
			return c.buildIntent("write", "create_event", 0.7, map[string]interface{}{
				"title": userInput,
			})
		}
	}

	// Update intents
	updateKeywords := []string{"change", "modify", "update", "alter", "edit", "改", "修改", "更新"}
	for _, kw := range updateKeywords {
		if strings.Contains(lower, kw) {
			return c.buildIntent("write", "update_event", 0.7, map[string]interface{}{
				"description": userInput,
			})
		}
	}

	// Reschedule intents
	rescheduleKeywords := []string{"move", "reschedule", "delay", "postpone", "advance", "延後", "提前", "挪到"}
	for _, kw := range rescheduleKeywords {
		if strings.Contains(lower, kw) {
			return c.buildIntent("write", "reschedule_event", 0.75, map[string]interface{}{
				"description": userInput,
			})
		}
	}

	// Delete intents
	deleteKeywords := []string{"delete", "remove", "cancel", "drop", "刪除", "取消", "移除"}
	for _, kw := range deleteKeywords {
		if strings.Contains(lower, kw) {
			return c.buildIntent("write", "delete_event", 0.75, map[string]interface{}{
				"description": userInput,
			})
		}
	}

	// Available slots
	slotsKeywords := []string{"available", "free", "open", "slot", "time", "空檔", "有空", "什麼時候有空"}
	for _, kw := range slotsKeywords {
		if strings.Contains(lower, kw) && strings.Contains(lower, "when") || strings.Contains(lower, "時候") {
			return c.buildIntent("query", "get_available_slots", 0.7, map[string]interface{}{
				"duration": 60, // default 1 hour
			})
		}
	}

	// Confirmation intents
	confirmKeywords := []string{"yes", "confirm", "approve", "ok", "alright", "sure", "是", "確認", "可以", "好"}
	rejectKeywords := []string{"no", "cancel", "reject", "decline", "不", "不要", "取消"}

	if matchKeywords(lower, confirmKeywords) {
		return c.buildIntent("confirmation", "approve", 0.8, make(map[string]interface{}))
	}

	if matchKeywords(lower, rejectKeywords) {
		return c.buildIntent("confirmation", "reject", 0.8, make(map[string]interface{}))
	}

	// Default to general conversation
	return c.buildIntent("query", "general", 0.5, map[string]interface{}{
		"text": userInput,
	})
}

// classifyWithLLM uses LLM for intent classification
func (c *SimpleIntentClassifier) classifyWithLLM(
	ctx context.Context,
	userInput string,
	conversationHistory []Message,
) (*Intent, error) {
	prompt := fmt.Sprintf(`Analyze the user input and classify it into one of these intents:
1. "query" - user wants information (get events, check availability, etc.)
2. "write" - user wants to modify calendar (create, update, delete, reschedule)
3. "confirmation" - user is responding to a confirmation request (yes/no)
4. "general" - general conversation

User input: %s

Respond with a JSON object:
{
  "type": "<intent_type>",
  "action": "<specific_action>",
  "confidence": <0.0-1.0>,
  "parameters": {
    "<key>": "<value>"
  },
  "requires_approval": <true_or_false>
}

Valid actions for each type:
- query: search, get_events, get_available_slots, get_event_details, query_history
- write: create_event, update_event, delete_event, reschedule_event
- confirmation: approve, reject
- general: chat

Generate only the JSON, no other text.`, userInput)

	response, err := c.llm.Generate(ctx, prompt)
	if err != nil {
		// Fall back to rule-based classification
		return c.classifyByRules(userInput), nil
	}

	// Parse JSON response
	var intent Intent
	if err := json.Unmarshal([]byte(response), &intent); err != nil {
		// Fall back to rule-based classification
		return c.classifyByRules(userInput), nil
	}

	return &intent, nil
}

// buildIntent constructs an Intent object
func (c *SimpleIntentClassifier) buildIntent(
	intentType, action string,
	confidence float64,
	parameters map[string]interface{},
) *Intent {
	return &Intent{
		Type:             intentType,
		Action:           action,
		Confidence:       confidence,
		Parameters:       parameters,
		RequiresApproval: intentType == "write",
	}
}

// matchKeywords checks if any keyword is in the text
func matchKeywords(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// SimpleResponseGenerator generates responses using LLM
type SimpleResponseGenerator struct {
	llm LLMInterface
}

// NewSimpleResponseGenerator creates a new response generator
func NewSimpleResponseGenerator(llm LLMInterface) *SimpleResponseGenerator {
	return &SimpleResponseGenerator{
		llm: llm,
	}
}

// Generate creates a natural language response
func (g *SimpleResponseGenerator) Generate(
	ctx context.Context,
	intent *Intent,
	toolResponses map[string]*ToolResponse,
) (string, error) {
	if g.llm == nil {
		return "I'm here to help with your calendar. What would you like to do?", nil
	}

	toolData, _ := json.MarshalIndent(toolResponses, "", "  ")
	prompt := fmt.Sprintf(`Generate a natural, friendly response for a calendar assistant user.

Intent: %s
Action: %s
Tool Responses: %s

The response should:
- Be concise and helpful
- Answer the user's question or confirm the action
- Be in the same language as the user's input (English or Chinese)
- Include relevant details but not be overwhelming

Generate only the response text, no additional formatting.`, intent.Type, intent.Action, string(toolData))

	response, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		return "I'll help you with that. Could you provide more details?", nil
	}

	return response, nil
}

// GenerateConfirmationPrompt creates a confirmation message
func (g *SimpleResponseGenerator) GenerateConfirmationPrompt(
	ctx context.Context,
	approval *PendingApproval,
) (string, error) {
	if g.llm == nil {
		return fmt.Sprintf("Please confirm: %s\n\nReply 'yes' or 'no'", approval.Summary), nil
	}

	prompt := fmt.Sprintf(`Generate a friendly confirmation prompt for a calendar assistant.

Change: %s
Expires at: %v

The prompt should:
- Be clear and concise
- Explain what will happen
- Ask for confirmation
- Include the expiration time

Generate only the prompt text.`, approval.Summary, approval.ExpiresAt)

	response, err := g.llm.Generate(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("Please confirm: %s\n\nReply 'yes' or 'no'", approval.Summary), nil
	}

	return response, nil
}
