# Bidirectional Two-Service Architecture Implementation

## Overview

Orbit-Orbi now implements a **bidirectional two-service gRPC architecture**:

### Service Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Core CLI (Client)                        │
│                    (localhost:50052)                          │
│                                                               │
│  - Sends user messages to Agent                              │
│  - Receives responses from Agent                             │
│  - Monitors Agent health/state                               │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ gRPC AgentService
                       │ ProcessMessage()
                       │ GetAgentState()
                       │
┌──────────────────────▼──────────────────────────────────────┐
│                   Orbi Agent Server                          │
│                  (localhost:50052)                           │
│                                                               │
│  - Implements AgentService                                   │
│  - Processes user messages with LangChain                    │
│  - Tracks sessions and state                                 │
└──────────────┬──────────────────────────┬───────────────────┘
               │                          │
               │ gRPC CalendarService     │
               │                          │
        ┌──────▼──────────────────────────▼──────┐
        │   External Calendar Service (Server)    │
        │        (localhost:50051)                │
        │                                         │
        │  - Manages calendar events              │
        │  - Provides calendar operations         │
        └─────────────────────────────────────────┘
```

## Two-Way Service Flow

### 1. **Core → Agent (User Input)**
- **Direction**: Client calls Server
- **Service**: `AgentService`
- **Methods**:
  - `ProcessMessage(message, session_id)` → Returns AI response
  - `GetAgentState()` → Returns Agent health/readiness
- **Use Case**: Core CLI sends user queries to Agent

### 2. **Agent → Calendar (Internal Processing)**
- **Direction**: Client calls Server
- **Service**: `CalendarService`
- **Methods**:
  - `CreateEvent(details)` → Creates event
  - `GetEvents(time_range)` → Retrieves events
  - `UpdateEvent(id, changes)` → Updates event
  - `DeleteEvent(id)` → Deletes event
  - `GetAvailableSlots(range, duration)` → Finds free slots
- **Use Case**: Agent needs calendar data to respond to user queries

## File Structure

### New/Updated Files

```
pkg/orbi/
├── agent.go          (existing - core agent logic)
├── server.go         (NEW - AgentService gRPC server implementation)

pkg/grpcclient/
├── client.go         (existing - CalendarService client)
├── agent.go          (NEW - AgentService client for Core CLI)

cmd/
├── orbi/
│   └── main.go       (UPDATED - now starts gRPC server + optional CLI)
├── core/
│   └── main.go       (NEW - CLI client that connects to Agent via gRPC)

proto/
└── calendar.proto    (UPDATED - added AgentService + message types)
```

## Running the Two-Service Architecture

### Option 1: Agent in Interactive Mode (for development)

```bash
# Terminal 1: Start Agent (with interactive CLI)
AGENT_MODE=interactive make run
# or
make run
```

The Agent will:
- Accept gRPC connections on port 50052
- Also provide an interactive CLI for testing
- Connect to Calendar Service on port 50051

### Option 2: Agent in Server Mode + Core CLI

```bash
# Terminal 1: Start Agent (server-only, no CLI)
AGENT_MODE=server make run-server

# Terminal 2: Start Core CLI (connects to Agent)
make run-core
```

Flow:
- Agent listens on `localhost:50052` (no interactive CLI)
- Core CLI connects to Agent on `localhost:50052`
- You interact via Core CLI
- Agent internally calls Calendar Service on `localhost:50051`

### Option 3: Both with make

```bash
# See available targets
make run-all
```

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `AGENT_MODE` | `interactive` | `interactive` (with CLI) or `server` (no CLI) |
| `AGENT_SERVICE_ADDR` | `localhost:50052` | Address Agent listens on |
| `CALENDAR_SERVICE_ADDR` | `localhost:50051` | Address of Calendar Service |
| `OPENAI_API_KEY` | (required) | OpenAI API key for LLM |
| `OPENAI_MODEL` | `gpt-3.5-turbo` | LLM model to use |

## Building

### Build both Agent and Core CLI

```bash
make build-all
# Outputs: bin/orbi (Agent), bin/core (Core CLI)
```

### Build individually

```bash
make build       # Build Agent
make build-core  # Build Core CLI
```

## Proto Generation

Before running, regenerate the protobuf Go code:

```bash
make proto
```

Or manually:

```powershell
$env:PATH = "$env:PATH;$(go env GOPATH)\bin"
protoc --go_out=. --go_opt=paths=source_relative `
       --go-grpc_out=. --go-grpc_opt=paths=source_relative `
       proto\calendar.proto
```

## Code Examples

### Agent Server (pkg/orbi/server.go)

```go
type AgentServer struct {
    pb.UnimplementedAgentServiceServer
    agent *Agent
    sessions map[string]*SessionState
}

func (s *AgentServer) ProcessMessage(ctx context.Context, 
    req *pb.ProcessMessageRequest) (*pb.ProcessMessageResponse, error) {
    // Process via agent.Chat()
    // Track session
    // Return response
}
```

### Core CLI Client (cmd/core/main.go)

```go
client, _ := grpcclient.NewAgentClient("localhost:50052")
state, _ := client.GetAgentState(ctx, sessionID)
response, _ := client.ProcessMessage(ctx, userInput, sessionID)
```

### Agent Main (cmd/orbi/main.go)

```go
// Create gRPC server
server := grpc.NewServer()
agentServer := orbi.NewAgentServer(agent)
pb.RegisterAgentServiceServer(server, agentServer)

// Start server
go server.Serve(listener)

// If interactive, also run CLI
if interactive {
    // Interactive loop
}
```

## Testing the Setup

### 1. Start Calendar Service (external)

```bash
go run ./examples/calendar-service/main.go
# Listens on localhost:50051
```

### 2. Start Orbi Agent (server mode)

```bash
$env:AGENT_MODE = "server"
$env:OPENAI_API_KEY = "your-api-key"
make run-server
# Listens on localhost:50052
```

### 3. Start Core CLI

```bash
make run-core
# Connects to localhost:50052
```

### 4. Interact

```
You: Schedule a meeting tomorrow at 2pm
Orbi: [Agent processes, calls Calendar Service, returns response]
Orbi: I've scheduled your meeting for tomorrow at 2:00 PM.
```

## Architecture Benefits

✅ **Separation of Concerns**: Agent handles reasoning, Calendar handles data  
✅ **Scalability**: Services can be on different machines  
✅ **Reusability**: Any client can call Agent or Calendar services  
✅ **Flexibility**: Run Agent interactive (dev) or server (prod)  
✅ **Testing**: Mock services independently  
✅ **Session Tracking**: Core CLI and Agent track conversation sessions  

## Troubleshooting

### "Failed to connect to agent"
- Ensure Agent is running: `make run-server`
- Check `AGENT_SERVICE_ADDR` env var
- Verify ports: `netstat -an | findstr 50052`

### "Agent not ready"
- Check `OPENAI_API_KEY` is set
- Verify Calendar Service is running on `CALENDAR_SERVICE_ADDR`

### Proto compilation errors
- Run: `make proto`
- Ensure `protoc` is on PATH
- Use `./run.ps1` to auto-download and configure tools

## Next Steps

1. Run `make proto` to regenerate proto files
2. Set your `OPENAI_API_KEY` env var
3. Run Calendar Service: `go run ./examples/calendar-service/main.go`
4. Run Agent: `make run-server`
5. Run Core CLI: `make run-core`
6. Start chatting!
