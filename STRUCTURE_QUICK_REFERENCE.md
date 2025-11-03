# Quick Reference Guide

## Component Overview

| Component | Purpose | Input | Output |
|-----------|---------|-------|--------|
| PlannerChatbotAgent | Orchestrator | User message | Agent response |
| RetrievalAgent | Query database | Intent | Summary |
| UpdateAgent | Modify calendar | Intent + approval | Confirmation |
| ConfirmationAgent | Validate changes | Approval | Summary |
| IntentClassifier | Parse intent | User message | Intent object |
| ResponseGenerator | Generate text | Intent + data | Natural language |

## Core Data Types

```go
// User intent
Intent{
  Type: "query|write|confirmation",
  Action: string,
  Confidence: float64,
  Parameters: map[string]interface{},
  RequiresApproval: bool,
}

// Conversation context
ConversationContext{
  SessionID: string,
  UserID: string,
  MessageHistory: []Message,
  PendingApprovals: map[string]*PendingApproval,
  AuditLogs: []AuditLog,
}

// Pending approval
PendingApproval{
  ID: string,
  ChangeType: "create|update|delete",
  Summary: string,
  ExpiresAt: time.Time,
  Approvals: []string,
}
```

## Common Operations

### Create Session
```go
session := planner.CreateSession("user-123")
```

### Process User Input
```go
response, err := planner.Process(ctx, "When is my next meeting?", session)
```

### Get Session History
```go
messages, _ := planner.GetSessionHistory(ctx, session.SessionID, 10)
```

### Get Chat State
```go
state := planner.GetChatState(session)
// state.PendingApproval shows if waiting for user confirmation
```

## Intent Classification

### Query Intents
```
User input patterns → Intent classification
"When...", "What...", "Show..." → query/search
"有", "什麼", "幾時" (Chinese) → query/search
```

### Write Intents
```
"Create", "New", "Schedule..." → write/create_event
"Change", "Update", "Modify..." → write/update_event
"Delete", "Remove", "Cancel..." → write/delete_event
"Move", "Reschedule", "延後..." → write/reschedule_event
```

### Confirmation Intents
```
"Yes", "Confirm", "OK..." → confirmation/approve
"No", "Cancel", "不..." → confirmation/reject
```

## Approval Workflow

### Draft Stage (UpdateAgent)
```go
// UpdateAgent creates pending approval
approval := &PendingApproval{
  ID: uuid.New().String(),
  ChangeType: "update",
  Summary: "I'll move Sprint Planning from 10am to 11am",
  ExpiresAt: time.Now().Add(5 * time.Minute),
}
```

### Validation Stage (ConfirmationAgent)
```go
// ConfirmationAgent validates
valid, reason, _ := validator.ValidateChange(ctx, "update", old, new)
conflicts, _ := validator.CheckConflicts(ctx, eventData)
```

### Confirmation Request (PlannerAgent)
```
Response: "🔒 Confirmation Required: I'll move Sprint Planning..."
Awaiting user response...
```

### Execution Stage (UpdateAgent)
```go
// User says "yes"
// UpdateAgent.ApplyPendingApproval() executes
// AuditTrail logs the action
```

## Error Handling

### Validation Errors
```
User: "Create meeting at 10am"
Issue: Time is in the past
Response: "Cannot create event: Time cannot be in the past"
```

### Conflict Errors
```
User: "Create meeting 2-3pm tomorrow"
Issue: "1:1 Sync" already 2-3pm
Response: "⚠️ Conflict detected: '1:1 Sync' 2-3pm"
```

### Approval Expiration
```
User: "yes" (after 5+ minutes)
Issue: Approval expired
Response: "⏰ Approval expired. Please request again."
```

## Testing Checklist

- [ ] Test query operations (no DB mutations)
- [ ] Test create operations (with approval)
- [ ] Test update operations (with conflict check)
- [ ] Test delete operations (confirmation required)
- [ ] Test approval expiration
- [ ] Test Chinese and English inputs
- [ ] Test error scenarios
- [ ] Test session cleanup

## Performance Tips

1. **Reuse Sessions**: Don't create new session per message
2. **Limit Context**: Keep context window to ~20 messages
3. **Cache Intents**: Use memory.SaveIntent() for similar patterns
4. **Async Logging**: Log to audit trail asynchronously
5. **Connection Pooling**: Reuse DB connections

## Deployment Checklist

- [ ] Implement PostgreSQL RetrievalTool
- [ ] Implement PostgreSQL UpdateTool
- [ ] Setup AuditTrail to database
- [ ] Configure LLM (OpenAI API key)
- [ ] Setup session memory (Redis or persistent)
- [ ] Add error monitoring
- [ ] Load test
- [ ] Security review

## Language Support

```go
// Automatic language detection
"When is..." → Responds in English
"幾時..." → Responds in Chinese

// In ResponseGenerator
prompt := fmt.Sprintf(`Respond in the same language as user input.`)
```

## Integration Points

### With Flutter App
```
WebSocket → Go Backend → PlannerAgent.Process() → Response
```

### With PostgreSQL
```
RetrievalTool → SELECT queries
UpdateTool → INSERT/UPDATE/DELETE with transactions
AuditTrail → Append-only logs
```

### With gRPC Calendar Service
```
UpdateTool → Calls calendar service gRPC methods
```

## Configuration Options

```go
// Session configuration
session.ContextWindow = 20      // Max messages in context
session.CreatedAt = time.Now()
session.LastActivityAt = time.Now()

// Approval configuration
approval.ExpiresAt = time.Now().Add(5 * time.Minute)
approval.RequiredApprovals = 1

// Agent configuration
planner.maxIterations = 5
updateAgent.SetRequiresConfirmation(true)
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Intent not recognized | Add pattern to SimpleIntentClassifier |
| Missing approval | Check UpdateAgent.Handle() creates PendingApproval |
| Memory growing | Implement session cleanup after timeout |
| Slow responses | Check LLM latency or add caching |
| Conflicts not detected | Verify validator.CheckConflicts() implementation |

## Common Patterns

### Pattern 1: Simple Query Flow
```
Query Intent → RetrievalAgent → Return summary
```

### Pattern 2: Update with Confirmation
```
Write Intent → UpdateAgent (draft) → Confirmation message → User approval → Execute
```

### Pattern 3: Multi-turn Conversation
```
Turn 1: Query → Response with context
Turn 2: Update → Confirmation request
Turn 3: Approval → Execute and confirm
```

## API Reference

### PlannerChatbotAgent
```go
Process(ctx, userInput, context) (string, error)
CreateSession(userID) *ConversationContext
GetSessionHistory(ctx, sessionID, limit) ([]Message, error)
ClearSession(ctx, sessionID) error
RegisterSubAgent(agent SubAgent) error
RegisterTool(tool Tool) error
```

### SubAgent (interface)
```go
Name() string
Handle(ctx, intent, context) (string, error)
ShouldHandle(intent) bool
```

### Tool (interface)
```go
Name() string
Description() string
Execute(ctx, params) (*ToolResponse, error)
RequiresApproval() bool
```

## Examples Directory

See `examples.go` for:
- ExampleUsage() - Basic setup
- ExampleMultiTurnConversation() - Multi-turn flow
- UseCase1_SimpleQuery() - Query operations
- UseCase2_ScheduleNewEvent() - Create with approval
- UseCase3_RescheduleWithConflictDetection() - Conflict handling
- UseCase4_ChineseConversation() - Chinese support

## Next Steps

1. Implement PostgreSQL tools (RetrievalTool, UpdateTool)
2. Connect to gRPC calendar service
3. Setup WebSocket for Flutter integration
4. Add comprehensive error handling
5. Deploy to staging environment
6. Load test and optimize
7. Production rollout

## Key Files

- `types.go` - Core interfaces and types
- `planner_agent.go` - Main orchestrator
- `retrieval_agent.go` - Read operations
- `update_agent.go` - Write operations
- `confirmation_agent.go` - Safety validation
- `memory_and_classifier.go` - Intent parsing
- `llm.go` - LLM abstraction
- `examples.go` - Usage examples
- `IMPLEMENTATION_GUIDE.md` - Detailed guide
- `README.md` - Full documentation
