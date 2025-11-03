# 🌐 Structured LangChain Flow - Smart Calendar Agent

## Overview

This implementation provides a **production-ready multi-agent architecture** for Orbit's 智能行事曆 (Smart Calendar). It follows the **Agent-Tool-Data loop pattern** with strict separation of concerns, safety-first design, and extensibility at its core.

## Key Features

✅ **Multi-Agent Architecture** - Specialized agents for different responsibilities  
✅ **Safety-First Design** - User confirmation gates for destructive operations  
✅ **Conversation Memory** - Persistent session management with audit trails  
✅ **LLM Neutrality** - Drop-in replacement for different LLMs (OpenAI, Gemini, vLLM)  
✅ **Bilingual Support** - Seamless English and Chinese conversation  
✅ **Conflict Detection** - Automatic scheduling conflict warnings  
✅ **Extensible Tools** - Easy to add new tools and integrations  
✅ **Full Audit Logging** - Complete history of all operations  

## Architecture at a Glance

```
┌─────────────────────────────────────────┐
│   User Interaction Layer                │
│   (WebSocket ← Flutter App)             │
└─────────────────┬───────────────────────┘
                  ↓
┌─────────────────────────────────────────┐
│   PlannerChatbotAgent                   │
│   (Primary Orchestrator)                │
└────┬────────────────────────┬───────────┘
     ↓                        ↓
┌──────────────┐    ┌────────────────┐
│ Retrieval    │    │    Update      │
│ Agent        │    │    Agent       │
│ (Read-only)  │    │ (Confirmation) │
└──────┬───────┘    └────────┬───────┘
       ↓                     ↓
   Retrieval Tool       Update Tool
   (PostgreSQL)         (Internal APIs)
       ↓                     ↓
    Data Layer
    ┌──────────────────────────────┐
    │ PostgreSQL + Audit Trail     │
    └──────────────────────────────┘
```

## Agents Explained

### 🔍 RetrievalAgent
- **Role**: Data lookup specialist
- **Responsibility**: Query calendar events, find available slots, retrieve event details
- **Access**: Read-only to database
- **Example**: "When is my next meeting?" → Queries database → Summarizes with LLM

### ✏️ UpdateAgent
- **Role**: Transaction executor
- **Responsibility**: Create, update, delete events with user confirmation
- **Access**: Write operations with approval gates
- **Example**: "Move meeting to 3pm" → Drafts change → Requests approval → Executes if approved

### ✅ ConfirmationAgent
- **Role**: Safety guardrail
- **Responsibility**: Validate changes, detect conflicts, generate confirmations
- **Access**: No database access, pure LLM reasoning
- **Example**: Checks for scheduling conflicts, generates human-readable summaries

### 🎯 PlannerChatbotAgent
- **Role**: Primary orchestrator
- **Responsibility**: Parse intent, route to sub-agents, manage conversation state
- **Access**: All tools via sub-agents
- **Example**: User input → Classify intent → Route to appropriate agent → Generate response

## Flow Examples

### Example 1: Simple Query (No Confirmation)

```
User: "When is my next meeting?"
  ↓
Classify: query/search
  ↓
Route to RetrievalAgent
  ↓
Query PostgreSQL
  ↓
Response: "Your next meeting is Sprint Planning at 2:00 PM"
```

### Example 2: Update with Confirmation

```
User: "幫我把明天的會議延後一小時"
  ↓
Classify: write/reschedule_event
  ↓
Route to UpdateAgent
  ├─ Draft change
  ├─ Check conflicts
  └─ Create PendingApproval
  ↓
Response: "🔒 I'll move Sprint Planning from 10am to 11am. Please confirm. [Approval ID: abc123]"
  ↓
User: "yes"
  ↓
Classify: confirmation/approve
  ↓
UpdateAgent executes change
  ├─ Call update tool
  ├─ Log to audit trail
  └─ Update database
  ↓
Response: "✅ Done! Sprint Planning is now at 11am"
```

### Example 3: Conflict Detection

```
User: "Create meeting tomorrow 2-3pm"
  ↓
UpdateAgent receives request
  ├─ ConfirmationAgent checks conflicts
  └─ Found: "1:1 Sync" 2-3pm
  ↓
Response: "⚠️ Conflict: '1:1 Sync' already scheduled 2-3pm. Try a different time."
```

## File Structure

```
pkg/orbi/
├── types.go                    # Core interfaces and data structures
├── llm.go                      # LLM abstraction layer
├── retrieval_agent.go          # Retrieval Agent implementation
├── update_agent.go             # Update Agent implementation
├── confirmation_agent.go       # Confirmation Agent implementation
├── planner_agent.go            # Main Planner Agent
├── memory_and_classifier.go    # Memory management & intent classification
├── examples.go                 # Usage examples and mocks
├── flow.md                     # High-level architecture documentation
├── IMPLEMENTATION_GUIDE.md     # Detailed implementation guide
└── README.md                   # This file
```

## Quick Start

### 1. Setup LLM

```go
import "github.com/tmc/langchaingo/llms/openai"

llm, _ := openai.New(openai.WithToken(os.Getenv("OPENAI_API_KEY")))
```

### 2. Create Components

```go
memory := orbi.NewInMemoryAgentMemory()
classifier := orbi.NewSimpleIntentClassifier(llm)
generator := orbi.NewSimpleResponseGenerator(llm)
auditTrail := &YourAuditTrail{}
```

### 3. Create Agents

```go
planner := orbi.NewPlannerChatbotAgent(llm, classifier, generator, memory, auditTrail)

// Register sub-agents
planner.RegisterSubAgent(orbi.NewRetrievalAgent(retrievalTool, llm, validator))
planner.RegisterSubAgent(orbi.NewUpdateAgent(updateTool, llm, validator, auditTrail))
planner.RegisterSubAgent(orbi.NewConfirmationAgent(llm, validator))
```

### 4. Process Conversations

```go
session := planner.CreateSession("user-123")
response, _ := planner.Process(ctx, "When is my next meeting?", session)
```

## Safety & Confirmation

### Approval Lifecycle

```
1. Draft Stage
   └─ UpdateAgent creates PendingApproval

2. Validation Stage
   └─ ConfirmationAgent validates & checks conflicts

3. User Confirmation
   └─ User responds "yes" or "no"

4. Execution
   └─ UpdateAgent.ApplyPendingApproval() called

5. Cleanup
   └─ PendingApproval removed, session updated
```

### Expiration Management

```go
// Approvals expire after 5 minutes (configurable)
approval := &PendingApproval{
  ExpiresAt: time.Now().Add(5 * time.Minute),
}

// Automatic cleanup on attempt to apply
if time.Now().After(approval.ExpiresAt) {
  return "Approval expired. Please request again."
}
```

## Conversation Memory

```go
// Automatically saved by PlannerAgent
- User messages
- Agent responses
- Intents and their classifications
- Audit logs of all operations

// Retrievable for context
history, _ := planner.GetSessionHistory(ctx, sessionID, 10)
```

## Tool Integration

### Implementing a Custom Tool

```go
type MyRetrievalTool struct {
  db *sql.DB
}

func (t *MyRetrievalTool) Name() string { return "my_tool" }
func (t *MyRetrievalTool) Description() string { return "..." }
func (t *MyRetrievalTool) GetEvents(ctx, start, end, filters) ([]map[string]interface{}, error) {
  // Query database
}

// Register with agent
planner.RegisterTool(myTool)
```

## Testing

### Using Mocks

```go
// All components have mock implementations for testing
mockLLM := orbi.NewMockLLM()
mockTool := &MockRetrievalTool{}
mockValidator := &MockValidator{}

planner := orbi.NewPlannerChatbotAgent(mockLLM, classifier, generator, memory, auditTrail)
```

### Running Examples

```go
// See examples.go for comprehensive examples
orbi.ExampleMultiTurnConversation()
orbi.UseCase2_ScheduleNewEvent(planner)
orbi.UseCase4_ChineseConversation(planner)
```

## Error Handling

### Graceful Degradation

```go
// If LLM fails, falls back to rule-based classification
intent, _ := classifier.Classify(ctx, userInput, history)

// If LLM is unavailable for summaries, returns raw data
response, _ := agent.Handle(ctx, intent, context)
```

### Validation Errors

```go
// Validator rejects invalid changes before execution
valid, reason, _ := validator.ValidateChange(ctx, "create", nil, data)
if !valid {
  return fmt.Sprintf("Cannot create: %s", reason), nil
}
```

## Performance Considerations

- **Connection Pooling**: Reuse database connections
- **Caching**: Cache intent classifications for similar patterns
- **Async Logging**: Non-blocking audit trail writes
- **Context Window**: Keep last N messages for LLM context (default: 20)

## Extensibility

### Adding New Sub-Agent

```go
type MyAgent struct{ ... }
func (a *MyAgent) Name() string { return "my-agent" }
func (a *MyAgent) ShouldHandle(intent *Intent) bool { ... }
func (a *MyAgent) Handle(ctx, intent, context) (string, error) { ... }

planner.RegisterSubAgent(myAgent)
```

### Adding New Tool Type

```go
type MyTool interface {
  Tool
  CustomMethod(ctx) (result, error)
}

// Implement and register
planner.RegisterTool(myTool)
```

### Supporting New Languages

```go
// Classifier detects language and responds accordingly
// ResponseGenerator adapts to user's language
// Just ensure LLM is multilingual-capable
```

## Bilingual Support (English & Chinese)

The system automatically detects and responds in the user's language:

```
User (Chinese): "幫我查一下明天的行程"
Orbi (Chinese): "我查到你明天有以下行程..."

User (English): "Show my meetings tomorrow"
Orbi (English): "You have the following meetings tomorrow..."
```

## Production Deployment Checklist

- [ ] Implement PostgreSQL connection pooling
- [ ] Set up Redis for distributed session management
- [ ] Configure audit logging to persistent storage
- [ ] Integrate with Flutter WebSocket layer
- [ ] Set up error monitoring (Sentry, DataDog)
- [ ] Load test with concurrent users
- [ ] Security review of LLM prompts
- [ ] Configure rate limiting and backpressure
- [ ] Set up distributed tracing
- [ ] Document API contracts

## API Contracts

### PlannerChatbotAgent.Process()

```go
func (a *PlannerChatbotAgent) Process(
  ctx context.Context,
  userInput string,
  context *ConversationContext,
) (string, error)
```

**Input**: User message (any language)  
**Output**: Agent response (same language as input)  
**Side Effects**: Saves to memory, logs to audit trail

## Troubleshooting

### Issue: "Intent not classified correctly"
**Solution**: Check SimpleIntentClassifier rules or feed examples to LLM

### Issue: "Missing approval confirmation"
**Solution**: Ensure UpdateAgent creates PendingApproval before execution

### Issue: "Memory growing unbounded"
**Solution**: Implement session cleanup after timeout (e.g., 1 hour)

## Contributing

To extend the system:

1. Implement the required interface (Tool, SubAgent, etc.)
2. Add unit tests using mocks
3. Update documentation
4. Create integration tests
5. Submit PR

## References

- **LangChain Documentation**: https://python.langchain.com/
- **Protocol Buffers**: `proto/calendar.proto`
- **gRPC Communication**: `pkg/grpcclient/`
- **OpenAI API**: https://platform.openai.com/docs/
- **PostgreSQL**: https://www.postgresql.org/docs/

## License

MIT

## Support

For issues or questions:
1. Check IMPLEMENTATION_GUIDE.md for detailed explanations
2. Review examples.go for usage patterns
3. Check existing unit tests
4. Open an issue on GitHub
