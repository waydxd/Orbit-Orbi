# Architecture Diagrams

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         User Interaction Layer                       │
│              Flutter App ↔ WebSocket ↔ Go Backend                   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  HTTP/WebSocket      │
                    │  Request Handler     │
                    └──────────┬───────────┘
                               │
                               ▼
        ┌──────────────────────────────────────────────────┐
        │  Intelligence Service (LangChain Core)          │
        │  ┌───────────────────────────────────────────┐  │
        │  │  Planner Chatbot Agent (Orchestrator)    │  │
        │  │  - Parse intent                           │  │
        │  │  - Route to sub-agents                    │  │
        │  │  - Manage conversation memory             │  │
        │  │  - Generate responses                     │  │
        │  └───┬──────────────┬──────────────┬─────────┘  │
        │      │              │              │            │
        │  ┌───▼──┐      ┌────▼────┐   ┌────▼─────────┐   │
        │  │Retri-│      │  Update  │   │Confirmation  │   │
        │  │eval  │      │  Agent   │   │   Agent      │   │
        │  │Agent │      └─────┬────┘   └──────────────┘   │
        │  │      │            │                           │
        │  └───┬──┘            │                           │
        │      │               │                           │
        │  Intent Classifier   └──────────┬────────────────┤
        │  Response Generator             │                │
        │                                 ▼                │
        │                          PendingApproval         │
        │                          Management              │
        └──────────────────────────┬──────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │                             │
             ┌──────▼────────┐          ┌────────▼─────┐
             │  Tool Layer   │          │  Memory      │
             ├───────────────┤          │  & Audit     │
             │ RetrievalTool │          └──────────────┘
             │ UpdateTool    │
             │ Validators    │
             └──────┬────────┘
                    │
        ┌───────────┴───────────┐
        │                       │
   ┌────▼──────┐        ┌──────▼────┐
   │ PostgreSQL│        │ gRPC APIs  │
   │           │        │            │
   │ Calendar  │        │ Calendar   │
   │ Events    │        │ Service    │
   │ Audit Log │        └────────────┘
   └───────────┘
```

## Conversation State Flow

```
                          ┌─────────────────────┐
                          │   New User Input    │
                          └──────────┬──────────┘
                                     │
                                     ▼
                    ┌────────────────────────────┐
                    │  1. Save to Message Store  │
                    └──────────┬─────────────────┘
                               │
                               ▼
                    ┌────────────────────────────┐
                    │  2. Classify Intent        │
                    │  (using LLM + rules)       │
                    └──────────┬─────────────────┘
                               │
                ┌──────────────┼──────────────┐
                │              │              │
                ▼              ▼              ▼
           ┌────────┐   ┌──────────┐   ┌───────────┐
           │ Query  │   │  Write   │   │Confirm.   │
           │        │   │          │   │           │
           └───┬────┘   └────┬─────┘   └─────┬─────┘
               │             │               │
               │             ▼               │
               │     ┌────────────────┐      │
               │     │ Draft Changes  │      │
               │     │ Create Pending │      │
               │     │ Approval       │      │
               │     └────┬───────────┘      │
               │          │                  │
               │          ▼                  │
               │   ┌─────────────────┐       │
               │   │ Request User    │       │
               │   │ Confirmation    │       │
               │   └────┬────────────┘       │
               │        │                    │
    ┌──────────▼────────▼────────┬──────────▼────────┐
    │                            │                    │
    ▼                            ▼                    ▼
┌────────┐                ┌──────────┐         ┌──────────┐
│Return  │                │ Wait for │         │Generate  │
│Summary │                │ 'yes'/'no'        │Confirm   │
└────────┘                └────┬─────┘         │Prompt    │
    │                          │               └────┬─────┘
    │                ┌─────────▼──────┐            │
    │                │                │            │
    │                ▼                ▼            │
    │          ┌───────────┐    ┌──────────┐      │
    │          │ Validate  │    │Generate  │      │
    │          │ & Check   │    │Response  │      │
    │          │Conflicts  │    └────┬─────┘      │
    │          └────┬──────┘         │            │
    │               │                │            │
    │               ▼                │            │
    │         ┌──────────┐           │            │
    │         │ Execute  │           │            │
    │         │ Change   │           │            │
    │         └────┬─────┘           │            │
    │              │                 │            │
    │              ▼                 │            │
    │         ┌──────────┐           │            │
    │         │ Log to   │           │            │
    │         │Audit     │           │            │
    │         └────┬─────┘           │            │
    │              │                 │            │
    └──────────────┴─────────────────┴────────────┘
                   │
                   ▼
         ┌────────────────────┐
         │ Send to User       │
         │ Save to Memory     │
         │ Update Context     │
         └────────┬───────────┘
                  │
                  ▼
           ┌────────────────┐
           │  Ready for     │
           │  Next Input    │
           └────────────────┘
```

## Agent Responsibility Matrix

```
┌────────────────────┬──────────────┬──────────────┬──────────────┐
│      Agent         │    Tools     │    Reads     │    Writes    │
├────────────────────┼──────────────┼──────────────┼──────────────┤
│ Planner Chatbot    │ All (via     │ Intent,      │ Memory,      │
│ Agent              │ delegates)   │ Context      │ Response     │
├────────────────────┼──────────────┼──────────────┼──────────────┤
│ Retrieval Agent    │ Retrieval    │ DB Events,   │ None         │
│                    │ Tool         │ Slots        │              │
├────────────────────┼──────────────┼──────────────┼──────────────┤
│ Update Agent       │ Update Tool  │ Event data   │ Pending      │
│                    │              │              │ Approval,    │
│                    │              │              │ Events       │
├────────────────────┼──────────────┼──────────────┼──────────────┤
│ Confirmation Agent │ Validator    │ Change data  │ None         │
│                    │ (read-only)  │              │ (validation) │
└────────────────────┴──────────────┴──────────────┴──────────────┘
```

## Data Flow Diagram: "Move Meeting to 3pm"

```
User Input
"幫我把明天的會議延後一小時"
    │
    ▼
┌──────────────────────────┐
│ Intent Classification    │
│ Type: "write"            │
│ Action: "reschedule"     │
│ Confidence: 0.85         │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ Planner Routes to        │
│ UpdateAgent              │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐      ┌─────────────────────┐
│ UpdateAgent              │──┐   │ RetrievalTool       │
│ 1. Find tomorrow's       │  │   │ Query: tomorrow's   │
│    meetings              │  └──▶│ events              │
│                          │      │ Result: 10-11am     │
└──────────┬───────────────┘      │ Sprint Planning     │
           │                       └─────────────────────┘
           ▼
┌──────────────────────────┐      ┌─────────────────────┐
│ UpdateAgent              │      │ Confirmation Agent  │
│ 2. Calculate new time    │      │ Validate new time   │
│    10am + 1hr = 11am     │─────▶│ Check conflicts     │
│                          │      │ ✓ No conflicts      │
└──────────┬───────────────┘      └─────────────────────┘
           │
           ▼
┌──────────────────────────┐
│ UpdateAgent              │
│ 3. Create PendingApproval│
│    ID: abc123            │
│    Expires: now + 5min   │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ Response Generator       │
│ Generate natural lang    │
│ "🔒 我將會把Sprint       │
│  Planning從10:00改到     │
│  11:00，是否確認？"      │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ Save to Memory:          │
│ - User message           │
│ - Agent response         │
│ - Intent cached          │
│ - Audit log              │
└──────────┬───────────────┘
           │
           ▼
    Response to User
    
    ─────────────────────────────────────
    
    User: "是"
    
    ─────────────────────────────────────
           │
           ▼
┌──────────────────────────┐
│ Classify Intent          │
│ Type: "confirmation"     │
│ Action: "approve"        │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ UpdateAgent              │
│ 1. Retrieve PendingAppr. │
│    ID: abc123            │
│ 2. Apply approval        │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ UpdateTool               │
│ Call: UpdateEvent(       │
│   event_id, new_time)    │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ PostgreSQL               │
│ UPDATE events            │
│ WHERE id = meeting_id    │
│ SET start_time = 11:00   │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ AuditTrail               │
│ Log: action=update       │
│ status=executed          │
└──────────┬───────────────┘
           │
           ▼
┌──────────────────────────┐
│ Response Generator       │
│ "✅ 完成！Sprint Planning│
│  已改到明天11:00"        │
└──────────┬───────────────┘
           │
           ▼
    Response to User
```

## Approval State Machine

```
                      ┌─────────────┐
                      │  DRAFT      │
                      │ (Created by │
                      │UpdateAgent) │
                      └──────┬──────┘
                             │
                    ┌────────▼────────┐
                    │  PENDING USER   │
                    │  CONFIRMATION   │
                    │  (Expires 5min) │
                    └────────┬────────┘
                             │
                ┌────────────┼────────────┐
                │            │            │
         ┌──────▼─┐    ┌─────▼────┐  ┌──▼─────┐
         │ EXPIRED │    │ APPROVED │  │REJECTED│
         │ (timeout)    │          │  │        │
         └─────────┘    └─────┬────┘  └────────┘
                               │
                        ┌──────▼──────┐
                        │  EXECUTING  │
                        │  (UpdateTool│
                        │   called)   │
                        └──────┬──────┘
                               │
                    ┌──────────┼───────────┐
                    │                      │
             ┌──────▼──────┐      ┌───────▼────┐
             │  SUCCESS    │      │   FAILED   │
             │ (Logged)    │      │ (Logged)   │
             └─────────────┘      └────────────┘
```

## Multi-Turn Conversation Example

```
Session ID: user-session-123
Start Time: 14:00:00

Turn 1:
  Input:  "Tomorrow's schedule?"
  Intent: {Type: "query", Action: "search"}
  Agent:  RetrievalAgent
  Output: "Tomorrow you have: Sprint Planning (10-11am), 1:1 (2pm)"
  Memory: Saved message + response

Turn 2:
  Input:  "Move the 10am meeting to 11am"
  Intent: {Type: "write", Action: "reschedule_event"}
  Agent:  UpdateAgent
  Output: "🔒 I'll move Sprint Planning from 10-11am to 11am-12pm. Confirm?"
  Memory: Saved message + response + pending approval
          
Turn 3:
  Input:  "yes"
  Intent: {Type: "confirmation", Action: "approve"}
  Agent:  UpdateAgent (applies approval)
  Output: "✅ Done! Sprint Planning moved to 11am"
  Memory: Saved message + response + cleared approval + logged action

Session State:
  Messages: 6 (3 user + 3 assistant)
  Intents: 3 (all cached)
  PendingApprovals: 0 (cleared after approval)
  AuditLogs: 2 (query logged, update logged)
```

## Component Dependencies

```
PlannerChatbotAgent
├── IntentClassifier
│   └── LLMInterface
├── ResponseGenerator
│   └── LLMInterface
├── RetrievalAgent
│   ├── RetrievalTool
│   ├── LLMInterface
│   └── ConfirmationValidator
├── UpdateAgent
│   ├── UpdateTool
│   ├── LLMInterface
│   ├── ConfirmationValidator
│   └── AuditTrail
├── ConfirmationAgent
│   ├── LLMInterface
│   └── ConfirmationValidator
└── AgentMemory
```

## Error Handling Flow

```
         ┌─────────────┐
         │  Error      │
         │  Occurs     │
         └──────┬──────┘
                │
                ▼
         ┌──────────────────┐
         │ Error Type?      │
         └──┬───┬───┬───┬───┘
            │   │   │   │
      ┌─────┘   │   │   └─────┐
      │         │   │         │
    ┌─┴─┐   ┌──┴┐┌─┴──┐   ┌──┴──┐
    │V  │   │C  ││DB  │   │Expi │
    │al │   │on ││Err │   │red  │
    │   │   │fl ││    │   │     │
    └─┬─┘   └──┬┘└──┬─┘   └──┬──┘
      │        │    │        │
      ▼        ▼    ▼        ▼
    "Canno"  "Conf" "DB Er" "Requ"
     t       lict    ror    est
    ────────────────────────────────
    Log to AuditTrail
    Save to Message History
    Return User-Friendly Message
```
