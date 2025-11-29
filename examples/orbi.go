package examples

import (
	"context"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-Orbi/pkg/orbi"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/agent"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/memory"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/types"
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

func (m *MockRetrievalTool) Name() string        { return "mock_retrieval_tool" }
func (m *MockRetrievalTool) Description() string { return "Mock retrieval tool" }
func (m *MockRetrievalTool) Execute(ctx context.Context, params map[string]interface{}) (*types.ToolResponse, error) {
	return &types.ToolResponse{Success: true, Data: map[string]interface{}{"count": 0}}, nil
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
) ([]types.Message, error) {
	return []types.Message{}, nil
}

// MockUpdateTool is a mock implementation of UpdateTool
type MockUpdateTool struct{}

func (m *MockUpdateTool) Name() string        { return "mock_update_tool" }
func (m *MockUpdateTool) Description() string { return "Mock update tool" }
func (m *MockUpdateTool) Execute(ctx context.Context, params map[string]interface{}) (*types.ToolResponse, error) {
	return &types.ToolResponse{Success: true, Data: map[string]interface{}{"id": "123"}}, nil
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
	approval *types.PendingApproval,
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
	intent *types.Intent,
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
	logs []types.AuditLog
}

func (m *MockAuditTrail) LogAction(ctx context.Context, log types.AuditLog) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *MockAuditTrail) GetAuditLog(ctx context.Context, sessionID string, limit int) ([]types.AuditLog, error) {
	return m.logs, nil
}

func (m *MockAuditTrail) VerifyAction(ctx context.Context, logID string) (bool, error) {
	return true, nil
}
