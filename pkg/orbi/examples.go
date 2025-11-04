package orbi

import (
	"context"
	"fmt"
	"time"
)

// ExampleUsage demonstrates how to use the multi-agent system
func ExampleUsage() {
	ctx := context.Background()

	// 1. Setup LLM (replace with actual implementation)
	mockLLM := NewMockLLM()
	mockLLM.AddResponse(
		"What is my next meeting?",
		"Your next meeting is Sprint Planning at 2:00 PM today.",
	)

	// 2. Setup memory and classifier
	memory := NewInMemoryAgentMemory()
	classifier := NewSimpleIntentClassifier(mockLLM)
	generator := NewSimpleResponseGenerator(mockLLM)

	// 3. Setup audit trail (mock)
	auditTrail := &MockAuditTrail{}

	// 4. Create planner agent (primary orchestrator)
	planner := NewPlannerChatbotAgent(mockLLM, classifier, generator, memory, auditTrail)

	// 5. Setup sub-agents
	retrievalAgent := NewRetrievalAgent(&MockRetrievalTool{}, mockLLM, &MockValidator{})
	updateAgent := NewUpdateAgent(&MockUpdateTool{}, mockLLM, &MockValidator{}, auditTrail)
	confirmationAgent := NewConfirmationAgent(mockLLM, &MockValidator{})

	// 6. Register sub-agents
	planner.RegisterSubAgent(retrievalAgent)
	planner.RegisterSubAgent(updateAgent)
	planner.RegisterSubAgent(confirmationAgent)

	// 7. Create conversation session
	session := planner.CreateSession("user-123")

	// 8. Process user input
	response, err := planner.Process(ctx, "What is my next meeting?", session)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", response)
}

// ExampleMultiTurnConversation demonstrates a multi-turn conversation
func ExampleMultiTurnConversation() {
	ctx := context.Background()

	// Setup
	mockLLM := NewMockLLM()
	memory := NewInMemoryAgentMemory()
	classifier := NewSimpleIntentClassifier(mockLLM)
	generator := NewSimpleResponseGenerator(mockLLM)
	auditTrail := &MockAuditTrail{}

	planner := NewPlannerChatbotAgent(mockLLM, classifier, generator, memory, auditTrail)

	// Register agents
	retrievalAgent := NewRetrievalAgent(&MockRetrievalTool{}, mockLLM, &MockValidator{})
	updateAgent := NewUpdateAgent(&MockUpdateTool{}, mockLLM, &MockValidator{}, auditTrail)
	confirmationAgent := NewConfirmationAgent(mockLLM, &MockValidator{})

	planner.RegisterSubAgent(retrievalAgent)
	planner.RegisterSubAgent(updateAgent)
	planner.RegisterSubAgent(confirmationAgent)

	session := planner.CreateSession("user-123")

	// Turn 1: Query
	fmt.Println("=== Turn 1: Query ===")
	response1, _ := planner.Process(ctx, "What's on my calendar tomorrow?", session)
	fmt.Printf("User: What's on my calendar tomorrow?\n")
	fmt.Printf("Orbi: %s\n\n", response1)

	// Turn 2: Update
	fmt.Println("=== Turn 2: Update (requires confirmation) ===")
	response2, _ := planner.Process(ctx, "Move my 10am meeting to 11am", session)
	fmt.Printf("User: Move my 10am meeting to 11am\n")
	fmt.Printf("Orbi: %s\n\n", response2)

	// Turn 3: Confirmation
	fmt.Println("=== Turn 3: Confirmation ===")
	response3, _ := planner.Process(ctx, "yes", session)
	fmt.Printf("User: yes\n")
	fmt.Printf("Orbi: %s\n\n", response3)
}

// ==================== Mock Implementations for Testing ====================

// MockRetrievalTool is a mock implementation of RetrievalTool
type MockRetrievalTool struct {
	events []map[string]interface{}
}

func (m *MockRetrievalTool) Name() string                        { return "mock_retrieval_tool" }
func (m *MockRetrievalTool) Description() string                { return "Mock retrieval tool" }
func (m *MockRetrievalTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResponse, error) {
	return &ToolResponse{Success: true, Data: map[string]interface{}{"count": 0}}, nil
}
func (m *MockRetrievalTool) RequiresApproval() bool { return false }

func (m *MockRetrievalTool) GetEvents(
	ctx context.Context,
	startTime, endTime time.Time,
	filters map[string]interface{},
) ([]map[string]interface{}, error) {
	return m.events, nil
}

func (m *MockRetrievalTool) GetAvailableSlots(
	ctx context.Context,
	startTime, endTime time.Time,
	duration time.Duration,
) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{"start": startTime, "end": startTime.Add(duration)},
	}, nil
}

func (m *MockRetrievalTool) GetEventDetails(
	ctx context.Context,
	eventID string,
) (map[string]interface{}, error) {
	return map[string]interface{}{"id": eventID, "title": "Sample Event"}, nil
}

func (m *MockRetrievalTool) QueryHistory(
	ctx context.Context,
	sessionID string,
	limit int,
) ([]Message, error) {
	return []Message{}, nil
}

// MockUpdateTool is a mock implementation of UpdateTool
type MockUpdateTool struct{}

func (m *MockUpdateTool) Name() string                        { return "mock_update_tool" }
func (m *MockUpdateTool) Description() string                { return "Mock update tool" }
func (m *MockUpdateTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResponse, error) {
	return &ToolResponse{Success: true, Data: map[string]interface{}{"id": "123"}}, nil
}
func (m *MockUpdateTool) RequiresApproval() bool { return true }

func (m *MockUpdateTool) CreateEvent(
	ctx context.Context,
	eventData map[string]interface{},
) (map[string]interface{}, error) {
	return map[string]interface{}{"id": "new-event", "status": "created"}, nil
}

func (m *MockUpdateTool) UpdateEvent(
	ctx context.Context,
	eventID string,
	updates map[string]interface{},
) (map[string]interface{}, error) {
	return map[string]interface{}{"id": eventID, "status": "updated"}, nil
}

func (m *MockUpdateTool) DeleteEvent(
	ctx context.Context,
	eventID string,
) error {
	return nil
}

func (m *MockUpdateTool) ApplyPendingApproval(
	ctx context.Context,
	approval *PendingApproval,
) error {
	return nil
}

// MockValidator is a mock implementation of ConfirmationValidator
type MockValidator struct{}

func (m *MockValidator) ValidateChange(
	ctx context.Context,
	changeType string,
	oldState, newState map[string]interface{},
) (bool, string, error) {
	return true, "", nil
}

func (m *MockValidator) GenerateHumanSummary(
	ctx context.Context,
	intent *Intent,
	changes map[string]interface{},
) (string, error) {
	return "Here's what will happen", nil
}

func (m *MockValidator) CheckConflicts(
	ctx context.Context,
	eventData map[string]interface{},
) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

// MockAuditTrail is a mock implementation of AuditTrail
type MockAuditTrail struct {
	logs []AuditLog
}

func (m *MockAuditTrail) LogAction(ctx context.Context, log AuditLog) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *MockAuditTrail) GetAuditLog(ctx context.Context, sessionID string, limit int) ([]AuditLog, error) {
	return m.logs, nil
}

func (m *MockAuditTrail) VerifyAction(ctx context.Context, logID string) (bool, error) {
	return true, nil
}

// ==================== Common Use Case Examples ====================

// UseCase1_SimpleQuery demonstrates a simple read-only query
func UseCase1_SimpleQuery(planner *PlannerChatbotAgent) string {
	ctx := context.Background()
	session := planner.CreateSession("user-456")

	response, _ := planner.Process(ctx, "When is my next meeting?", session)
	return response
}

// UseCase2_ScheduleNewEvent demonstrates creating a new event
func UseCase2_ScheduleNewEvent(planner *PlannerChatbotAgent) string {
	ctx := context.Background()
	session := planner.CreateSession("user-789")

	// First message asks to create event (ignore intermediate response)
	_, _ = planner.Process(ctx, "Schedule a meeting tomorrow at 3pm", session)

	// Second message confirms
	response2, _ := planner.Process(ctx, "yes", session)

	return response2
}

// UseCase3_RescheduleWithConflictDetection demonstrates conflict handling
func UseCase3_RescheduleWithConflictDetection(planner *PlannerChatbotAgent) string {
	ctx := context.Background()
	session := planner.CreateSession("user-abc")

	// User tries to move a meeting
	response, _ := planner.Process(ctx, "Move my 10am meeting to 2pm", session)

	return response
}

// UseCase4_ChineseConversation demonstrates Chinese language support
func UseCase4_ChineseConversation(planner *PlannerChatbotAgent) string {
	ctx := context.Background()
	session := planner.CreateSession("user-chn")

	// Chinese query (ignore intermediate responses)
	_, _ = planner.Process(ctx, "幫我查一下明天的行程", session)

	// Chinese reschedule request
	_, _ = planner.Process(ctx, "幫我把明天的會議延後一小時", session)

	// Chinese confirmation
	response3, _ := planner.Process(ctx, "可以", session)

	return response3
}

// ==================== Conversation State Tracking ====================

// ExampleSessionManagement demonstrates session management
func ExampleSessionManagement() {
	mockLLM := NewMockLLM()
	memory := NewInMemoryAgentMemory()
	classifier := NewSimpleIntentClassifier(mockLLM)
	generator := NewSimpleResponseGenerator(mockLLM)
	auditTrail := &MockAuditTrail{}

	planner := NewPlannerChatbotAgent(mockLLM, classifier, generator, memory, auditTrail)

	ctx := context.Background()

	// Create session
	session := planner.CreateSession("user-001")
	fmt.Printf("Session ID: %s\n", session.SessionID)
	fmt.Printf("Created at: %v\n", session.CreatedAt)

	// Process messages
	planner.Process(ctx, "What's tomorrow?", session)
	planner.Process(ctx, "Schedule meeting at 3pm", session)

	// Get chat state
	state := planner.GetChatState(session)
	fmt.Printf("Messages in session: %d\n", state.MessageCount)
	fmt.Printf("Last activity: %v\n", state.LastMessageTime)
	fmt.Printf("Pending approvals: %v\n", state.PendingApproval != nil)

	// Retrieve history
	history, _ := planner.GetSessionHistory(ctx, session.SessionID, 10)
	fmt.Printf("Total history records: %d\n", len(history))
}

// ==================== Error Handling Examples ====================

// ExampleErrorHandling demonstrates error handling
func ExampleErrorHandling(planner *PlannerChatbotAgent) {
	ctx := context.Background()
	session := planner.CreateSession("error-user")

	// Invalid intent
	response1, _ := planner.Process(ctx, "", session) // Empty input
	fmt.Printf("Empty input handling: %s\n", response1)

	// Expired approval
	session.PendingApprovals["expired"] = &PendingApproval{
		ID:        "expired",
		ExpiresAt: time.Now().Add(-1 * time.Minute), // Already expired
		Summary:   "Expired change",
	}

	// Attempt to apply expired approval
	response2, _ := planner.Process(ctx, "yes", session)
	fmt.Printf("Expired approval handling: %s\n", response2)
}

// ==================== Helper Functions ====================

// CreateFullyConfiguredPlannerAgent creates a complete, ready-to-use planner
func CreateFullyConfiguredPlannerAgent() *PlannerChatbotAgent {
	// Setup
	mockLLM := NewMockLLM()
	memory := NewInMemoryAgentMemory()
	classifier := NewSimpleIntentClassifier(mockLLM)
	generator := NewSimpleResponseGenerator(mockLLM)
	auditTrail := &MockAuditTrail{}

	// Create planner
	planner := NewPlannerChatbotAgent(mockLLM, classifier, generator, memory, auditTrail)

	// Register agents
	retrievalAgent := NewRetrievalAgent(&MockRetrievalTool{}, mockLLM, &MockValidator{})
	updateAgent := NewUpdateAgent(&MockUpdateTool{}, mockLLM, &MockValidator{}, auditTrail)
	confirmationAgent := NewConfirmationAgent(mockLLM, &MockValidator{})

	planner.RegisterSubAgent(retrievalAgent)
	planner.RegisterSubAgent(updateAgent)
	planner.RegisterSubAgent(confirmationAgent)

	return planner
}

// PrintConversationFlow prints a visualization of the conversation flow
func PrintConversationFlow() {
	fmt.Println(`
Conversation Flow Diagram:

User Input
    ↓
┌─────────────────────────┐
│  PlannerChatbotAgent    │
│  1. Save message        │
│  2. Classify intent     │
│  3. Route to sub-agent  │
└────────┬────────────────┘
         ↓
    ┌────┴────────┬──────────┬──────────┐
    ↓             ↓          ↓          ↓
┌────────┐  ┌──────────┐  ┌──────────┐
│Retrieval│  │  Update  │  │Confirm.  │
│ Agent   │  │  Agent   │  │  Agent   │
└────┬───┘  └──┬───────┘  └────┬─────┘
     ↓         ↓               ↓
   Query    Draft & Request  Validate
   DB       Approval         Changes
   ↓        ↓                ↓
  Data    PendingApproval   Summary
   ↓        ↓                ↓
   └────┬───┴────────────────┘
        ↓
┌─────────────────────────┐
│  ResponseGenerator      │
│  Generate response      │
└────────┬────────────────┘
         ↓
   User Response
`)
}
