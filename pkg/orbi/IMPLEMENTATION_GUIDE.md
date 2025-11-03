# Structured LangChain Flow Implementation Guide

## Overview

This implementation provides a complete multi-agent architecture for Orbit's 智能行事曆 (Smart Calendar). The system is built on the Agent-Tool-Data loop pattern with strict separation of concerns.

## Architecture Components

### 1. Core Interfaces (types.go)

Define the contracts that all agents and tools must follow:

```go
// SubAgent - Interface all agents implement
interface {
  Name() string
  Handle(ctx, intent, context) (string, error)
  ShouldHandle(intent) bool
}

// Tool interfaces
interface RetrievalTool { ... }  // Read-only database operations
interface UpdateTool { ... }     // Write operations with confirmation
```

### 2. Sub-Agents

#### RetrievalAgent (retrieval_agent.go)
- **Purpose**: Handle data queries
- **Tools**: RetrievalTool only
- **Responsibilities**:
  - Query events by time range
  - Find available slots
  - Retrieve event details
  - Search with multiple criteria

```go
agent := NewRetrievalAgent(retrievalTool, llm, validator)
// Handles: "When is my next meeting?"
// Handles: "Find available slots tomorrow"
```

#### UpdateAgent (update_agent.go)
- **Purpose**: Execute calendar changes
- **Tools**: UpdateTool only
- **Responsibilities**:
  - Draft changes
  - Request user confirmation
  - Execute approved changes
  - Log audit trail

```go
agent := NewUpdateAgent(updateTool, llm, validator, auditTrail)
// Handles: "Create meeting at 3pm"
// Handles: "Move tomorrow's 10am meeting to 11am"
```

#### ConfirmationAgent (confirmation_agent.go)
- **Purpose**: Validate and confirm changes
- **Responsibilities**:
  - Validate proposed changes
  - Check for conflicts
  - Generate human-readable summaries
  - Request user approval

```go
agent := NewConfirmationAgent(llm, validator)
// Validates all destructive operations
// Checks scheduling conflicts
```

#### PlannerChatbotAgent (planner_agent.go)
- **Purpose**: Primary orchestrator
- **Responsibilities**:
  - Parse user intent
  - Route to appropriate sub-agent
  - Manage conversation memory
  - Generate responses

```go
planner := NewPlannerChatbotAgent(llm, classifier, generator, memory, audit)
response, err := planner.Process(ctx, "幫我把明天的會議延後一小時", context)
```

### 3. LLM Integration (llm.go)

```go
// Interface allows swapping between different LLMs
interface LLMInterface {
  Generate(ctx, prompt) (string, error)
  GenerateWithOptions(ctx, prompt, opts) (string, error)
}

// Implementations
- SimpleLLMAdapter  // For OpenAI, Gemini, etc.
- MockLLM          // For testing
```

Usage:
```go
llm, err := openai.New(openai.WithToken(apiKey))
adapter := NewSimpleLLMAdapter(func(ctx context.Context, prompt string) (string, error) {
  // Wrap OpenAI client
  return llm.Call(ctx, prompt)
})
```

### 4. Memory Management (memory_and_classifier.go)

```go
// In-memory storage for conversation state
memory := NewInMemoryAgentMemory()

// Saves/retrieves messages and intents
memory.SaveMessage(ctx, sessionID, msg)
messages, _ := memory.GetMessages(ctx, sessionID, 10)
```

### 5. Intent Classification (memory_and_classifier.go)

```go
classifier := NewSimpleIntentClassifier(llm)

intent, err := classifier.Classify(ctx, "Create meeting tomorrow at 3pm", history)
// Returns: Intent{Type: "write", Action: "create_event", Confidence: 0.9, ...}
```

## Flow Examples

### Example 1: Query Flow (No Confirmation Needed)

```
User: "When is my next meeting?"
  ↓
PlannerAgent.Process()
  ├─ Classify intent → {type: "query", action: "search"}
  ├─ Route to RetrievalAgent
  │  ├─ Query database for next event
  │  └─ Generate natural language summary
  └─ Return response: "Your next meeting is at 3pm: Sprint Planning"
```

### Example 2: Update Flow (With Confirmation)

```
User: "Move tomorrow's 10am meeting to 11am"
  ↓
PlannerAgent.Process()
  ├─ Classify intent → {type: "write", action: "reschedule_event"}
  ├─ Route to UpdateAgent
  │  ├─ Draft change
  │  ├─ Create PendingApproval
  │  └─ Return: "🔒 I'll move Sprint Planning from 10am to 11am. Approval ID: abc123"
  │
  └─ Wait for user confirmation
      ├─ User says "yes"
      ├─ Classify intent → {type: "confirmation", action: "approve"}
      ├─ UpdateAgent.ApplyApproval()
      │  ├─ Call update tool
      │  ├─ Log to audit trail
      │  └─ Update database
      └─ Return: "✅ Done! Sprint Planning is now at 11am"
```

### Example 3: Conflict Detection

```
User: "Create meeting tomorrow at 2pm to 3pm"
  ↓
PlannerAgent → UpdateAgent
  ├─ Check for conflicts → Found: "1:1 Sync" 2-3pm
  ├─ ConfirmationAgent validates
  └─ Return: "⚠️ Conflict detected: '1:1 Sync' is already scheduled 2-3pm"
```

## Integration Guide

### Step 1: Set Up Components

```go
package main

import (
  "github.com/waydxd/Orbit-Orbi/pkg/orbi"
  "github.com/tmc/langchaingo/llms/openai"
)

// Initialize LLM
llm, _ := openai.New(openai.WithToken(os.Getenv("OPENAI_API_KEY")))
llmAdapter := orbi.NewSimpleLLMAdapter(func(ctx context.Context, prompt string) (string, error) {
  // Implement actual LLM call
  return llm.Call(ctx, prompt)
})

// Initialize memory
memory := orbi.NewInMemoryAgentMemory()

// Initialize classifier and generator
classifier := orbi.NewSimpleIntentClassifier(llmAdapter)
generator := orbi.NewSimpleResponseGenerator(llmAdapter)

// Initialize audit trail
auditTrail := &SimpleAuditTrail{} // Implement this
```

### Step 2: Create Tools

```go
// Implement RetrievalTool
retrievalTool := &MyRetrievalTool{
  db: database,
}

// Implement UpdateTool
updateTool := &MyUpdateTool{
  db: database,
  api: grpcClient,
}

// Implement ConfirmationValidator
validator := &MyValidator{
  db: database,
}
```

### Step 3: Create Agents

```go
// Sub-agents
retrieval := orbi.NewRetrievalAgent(retrievalTool, llmAdapter, validator)
update := orbi.NewUpdateAgent(updateTool, llmAdapter, validator, auditTrail)
confirmation := orbi.NewConfirmationAgent(llmAdapter, validator)

// Primary agent
planner := orbi.NewPlannerChatbotAgent(llmAdapter, classifier, generator, memory, auditTrail)

// Register sub-agents
planner.RegisterSubAgent(retrieval)
planner.RegisterSubAgent(update)
planner.RegisterSubAgent(confirmation)
```

### Step 4: Process User Input

```go
// Create session
session := planner.CreateSession("user-123")

// Process message
response, err := planner.Process(ctx, "幫我把明天的會議延後一小時", session)
```

## Safety & Confirmation Flow

### Approval Lifecycle

```
1. Draft Stage
   - UpdateAgent creates PendingApproval
   - Returns approval_id to user

2. Validation Stage
   - ConfirmationAgent validates changes
   - Checks for conflicts
   - Generates human-readable summary

3. User Confirmation
   - User responds "yes" or "no"
   - PlannerAgent routes to UpdateAgent

4. Execution
   - UpdateAgent.ApplyApproval() called
   - Tool executes actual changes
   - AuditTrail logs the action

5. Cleanup
   - PendingApproval removed
   - Session updated
```

### Approval Expiration

```go
// Approvals expire after 5 minutes
approval := &PendingApproval{
  ExpiresAt: time.Now().Add(5 * time.Minute),
}

// Check before applying
if time.Now().After(approval.ExpiresAt) {
  return "Approval expired. Please request again."
}
```

## Conversation Memory Management

```go
// Save user message
planner.Process() → memory.SaveMessage(sessionID, userMsg)

// Save intent for context
classifier.Classify() → memory.SaveIntent(sessionID, intent)

// Retrieve conversation history
history, _ := memory.GetMessages(ctx, sessionID, 10)

// Context window keeps last N messages for LLM context
contextWindow := min(len(history), 20)
```

## Error Handling

### Tool Failures

```go
// If retrieval fails
if err := tool.GetEvents(...) {
  return "", fmt.Errorf("failed to retrieve events: %w", err)
  // PlannerAgent catches and logs
}
```

### Validation Failures

```go
// Validator rejects invalid changes
valid, reason, _ := validator.ValidateChange(ctx, "create", nil, eventData)
if !valid {
  return fmt.Sprintf("Cannot create event: %s", reason), nil
}
```

### LLM Failures (Graceful Degradation)

```go
// If LLM fails, fall back to simple rules
response, err := a.llm.Generate(ctx, prompt)
if err != nil {
  // Fallback implementation
  return a.generateFallbackSummary(intent.Action, changes), nil
}
```

## Testing

### Unit Testing Example

```go
func TestRetrievalAgent(t *testing.T) {
  // Mock tool
  mockTool := &MockRetrievalTool{}
  mockTool.On("GetEvents").Return([]map[string]interface{}{...})

  // Mock LLM
  mockLLM := orbi.NewMockLLM()

  // Create agent
  agent := orbi.NewRetrievalAgent(mockTool, mockLLM, validator)

  // Test
  intent := &orbi.Intent{Type: "query", Action: "get_events"}
  response, _ := agent.Handle(context.Background(), intent, context)

  assert.NotEmpty(t, response)
}
```

### Integration Testing

```go
func TestCompleteFlow(t *testing.T) {
  // Setup all components
  planner := setupPlannerAgent()
  session := planner.CreateSession("user-123")

  // Step 1: Query
  response, _ := planner.Process(ctx, "Tomorrow's meetings?", session)
  assert.Contains(t, response, "meeting")

  // Step 2: Update with confirmation
  response, _ = planner.Process(ctx, "Move first meeting to 11am", session)
  assert.Contains(t, response, "🔒")

  // Step 3: Confirm
  response, _ = planner.Process(ctx, "yes", session)
  assert.Contains(t, response, "✅")
}
```

## Performance Considerations

### Caching
```go
// Cache intent classifications for similar requests
intentCache := make(map[string]*Intent)
```

### Connection Pooling
```go
// Reuse database connections
db := NewConnectionPool(10) // max 10 connections
```

### Async Processing
```go
// Long-running operations in background
go func() {
  auditTrail.LogAction(ctx, log)
}()
```

## Extensibility

### Adding New Tool

```go
type MyCustomTool struct {
  // implementation
}

func (t *MyCustomTool) Name() string { return "my_tool" }
func (t *MyCustomTool) Description() string { return "..." }
func (t *MyCustomTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResponse, error) {
  // implementation
}

// Register with agent
agent.RegisterTool(myCustomTool)
```

### Adding New Sub-Agent

```go
type MyAgent struct {
  // implementation
}

func (a *MyAgent) Name() string { return "my-agent" }
func (a *MyAgent) ShouldHandle(intent *Intent) bool {
  return intent.Action == "my_action"
}
func (a *MyAgent) Handle(ctx context.Context, intent *Intent, context *ConversationContext) (string, error) {
  // implementation
}

planner.RegisterSubAgent(myAgent)
```

## Next Steps

1. **Implement concrete tools** that interface with PostgreSQL and gRPC
2. **Add WebSocket layer** for real-time communication with Flutter app
3. **Implement audit logging** to PostgreSQL
4. **Add language detection** for automatic English/Chinese handling
5. **Create comprehensive tests** for all agents and flows
6. **Optimize performance** with caching and connection pooling
7. **Deploy** to production with monitoring
