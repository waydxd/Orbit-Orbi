# 🌐 Structured LangChain Flow Design for Orbit's 智能行事曆

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│             User Interaction Layer                          │
│  (Flutter App → WebSocket → Go Backend)                    │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│      Intelligence Service (LangChain Core)                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ Planner Chatbot Agent (Primary Orchestrator)        │   │
│  │  - Parse user intent                                │   │
│  │  - Route to sub-agents                              │   │
│  │  - Manage conversation memory                       │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────┬──────────────────────┬──────────────────────┘
               │                      │
    ┌──────────▼──────────┐  ┌────────▼─────────────┐
    │  Retrieval Agent    │  │  Update Agent       │
    │  - Query events     │  │  - Draft changes    │
    │  - Find slots       │  │  - Request confirm  │
    │  - Summarize data   │  │  - Execute API      │
    └────────┬────────────┘  └────────┬─────────────┘
             │                        │
             ▼                        ▼
    ┌─────────────────┐      ┌──────────────────┐
    │  Retrieval Tool │      │ Update Tool      │
    │  (PostgreSQL)   │      │ (Internal APIs)  │
    └────────┬────────┘      └─────────┬────────┘
             │                         │
             └──────────┬──────────────┘
                        ▼
            ┌──────────────────────────┐
            │  Confirmation Agent      │
            │  - Validate changes      │
            │  - Generate summaries    │
            │  - Safety guardrails     │
            └──────────────────────────┘
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                 Data Layer                                  │
│  ┌──────────────────────┐  ┌──────────────────────────┐   │
│  │  PostgreSQL Calendar │  │  Audit Logs             │   │
│  │  - Events            │  │  - Confirmations        │   │
│  │  - Availability      │  │  - Rollback History     │   │
│  │  - Metadata          │  │  - Change Trail         │   │
│  └──────────────────────┘  └──────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## 🧩 Agent Roles and Responsibilities

| Agent | Role | Responsibilities | Tools |
|-------|------|-----------------|-------|
| **Planner Chatbot Agent** | Primary Orchestrator | Parse intent, route requests, manage memory | Retrieval Tool, Update Tool |
| **Retrieval Agent** | Data Lookup | Query DB, find slots, summarize results | Retrieval Tool only |
| **Update Agent** | Transaction Executor | Draft changes, request confirmation, call APIs | Update Tool only |
| **Confirmation Agent** | Safety Guardrail | Validate changes, generate human-readable summaries | None (LLM reasoning) |

## 🔄 Flow Example: "幫我把明天的會議延後一小時"

### Step 1: Intent Parsing (Planner Agent)
```
User: "幫我把明天的會議延後一小時"
         ↓
Planner Agent detects: scheduling_update event
Action: Delegate to Retrieval Agent
```

### Step 2: Data Retrieval (Retrieval Agent)
```
Query: "Find all events on tomorrow"
Response: 
- Meeting 1: 10:00-11:00 "Sprint Planning"
- Meeting 2: 14:00-15:00 "1:1 Sync"
```

### Step 3: Change Drafting (Update Agent)
```
Proposed Change:
  Event: "Sprint Planning"
  Old:   10:00-11:00
  New:   11:00-12:00
```

### Step 4: Confirmation (Confirmation Agent)
```
Summary: "我將會把明天 10:00 的會議改到 11:00，是否確認？"
Awaiting: User confirmation
```

### Step 5: Execution (Update Agent)
```
User confirms ✓
→ Call internal API: UpdateEvent(id, new_time)
→ Update PostgreSQL
→ Log to audit trail
→ Return success
```

### Step 6: Closure (Planner Agent)
```
Response: "完成！我已把 Sprint Planning 改到明天 11:00"
```

## ⚙️ Design Principles

### 1. Microservices Separation
- **Go Backend**: Orchestrator + LangChain host
- **Database + APIs**: Isolated services
- **Modularity**: Each agent is independently testable

### 2. LLM Neutrality
- **Current**: Gemini API / OpenAI
- **Future**: Drop-in replacement (vLLM)
- **Implementation**: Interface-based LLM client

### 3. Safety-First
- **Confirmation Gateway**: No silent destructive updates
- **Audit Logging**: Every change tracked
- **User Approval**: Critical operations require consent
- **Rollback Support**: Ability to undo changes

### 4. Extensibility
- **Add New Tools**: Without changing core flow
- **Pluggable Integrations**: Email, Slack, Zoom, etc.
- **Custom Validators**: Domain-specific business rules
- **Memory Management**: Persistent conversation state

## 📋 Data Structures

### ConversationContext
```go
type ConversationContext struct {
    SessionID       string
    UserID          string
    MessageHistory  []Message
    IntentCache     map[string]Intent
    PendingApprovals []PendingApproval
    CreatedAt       time.Time
}
```

### PendingApproval
```go
type PendingApproval struct {
    ID              string
    ChangeType      string
    OldState        map[string]interface{}
    NewState        map[string]interface{}
    Summary         string
    ExpiresAt       time.Time
    RequiredApprovals int
}
```

### AuditLog
```go
type AuditLog struct {
    ID              string
    UserID          string
    Action          string
    ResourceType    string
    ResourceID      string
    OldValue        string
    NewValue        string
    Status          string // approved, rejected, pending
    Timestamp       time.Time
}
```

## 🛠️ Tool Specifications

### Retrieval Tool
- **Purpose**: Query PostgreSQL for calendar data
- **Methods**: GetEvents, GetAvailableSlots, GetEventDetails, QueryHistory
- **Access Control**: Read-only
- **Caching**: Short-lived cache for repeated queries

### Update Tool
- **Purpose**: Execute transactional updates
- **Methods**: CreateEvent, UpdateEvent, DeleteEvent, ApplyChanges
- **Confirmation**: All operations require pre-approval
- **Rollback**: Maintains transaction history

### Optional Integration Tools
- **EmailNotifier**: Send reminders and confirmations
- **SlackNotifier**: Post updates to Slack channels
- **ZoomLinkFinder**: Extract and insert Zoom links
- **AttendeeManager**: Handle RSVP and availability

## 📊 State Management

```
Conversation Flow State Machine:

  IDLE
    ↓
  LISTENING (user input)
    ↓
  INTENT_CLASSIFICATION
    ↓
  TOOL_SELECTION (which agent to route to)
    ↓
  DATA_RETRIEVAL (if needed)
    ↓
  CHANGE_DRAFT (if modification)
    ↓
  CONFIRMATION_PENDING (wait for user)
    ↓
  CHANGE_EXECUTION (apply updates)
    ↓
  AUDIT_LOGGING
    ↓
  RESPONSE_GENERATION
    ↓
  IDLE
```

## 🔐 Safety & Validation Layers

1. **Input Validation**: Sanitize user input, detect injection attacks
2. **Intent Validation**: Confirm parsed intent matches user request
3. **Change Validation**: Verify proposed changes are logical
4. **Conflict Detection**: Check for scheduling conflicts
5. **Permission Checking**: Verify user has edit permissions
6. **Confirmation Gateway**: User must approve destructive operations
7. **Audit Trail**: Log all operations for compliance

## 📈 Scalability Considerations

- **Memory Management**: Cache frequently accessed queries
- **Connection Pooling**: Reuse database connections
- **Rate Limiting**: Prevent abuse, implement backpressure
- **Async Processing**: Long-running operations in background
- **Distributed Tracing**: Track requests across services
- **Metrics Collection**: Monitor agent performance and errors

## 🎯 Phase Implementation

### Phase 1: Core Infrastructure
- [ ] Implement base agent types
- [ ] Create tool interfaces
- [ ] Build conversation context management
- [ ] Set up audit logging

### Phase 2: Primary Agents
- [ ] Implement Planner Chatbot Agent
- [ ] Implement Retrieval Agent
- [ ] Implement Update Agent
- [ ] Build tool wrappers

### Phase 3: Safety & Confirmation
- [ ] Implement Confirmation Agent
- [ ] Add user approval workflows
- [ ] Build validation layers
- [ ] Create audit trail system

### Phase 4: Integration & Polish
- [ ] WebSocket integration
- [ ] Error handling and recovery
- [ ] Performance optimization
- [ ] Comprehensive testing

