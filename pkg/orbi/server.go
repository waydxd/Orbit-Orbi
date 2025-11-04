package orbi

import (
	"context"
	"fmt"
	"sync"

	pb "github.com/waydxd/Orbit-Orbi/proto"
)

// AgentServer implements the AgentService gRPC service
type AgentServer struct {
	pb.UnimplementedAgentServiceServer
	agent       *Agent
	mu          sync.RWMutex
	sessions    map[string]*SessionState
	isReady     bool
	readyReason string
}

// SessionState tracks the state of a user session
type SessionState struct {
	SessionID string
	Created   int64
	LastUsed  int64
	Messages  []string
}

// NewAgentServer creates a new gRPC server for the Orbi agent
func NewAgentServer(agent *Agent) *AgentServer {
	return &AgentServer{
		agent:       agent,
		sessions:    make(map[string]*SessionState),
		isReady:     true,
		readyReason: "Agent initialized and ready",
	}
}

// ProcessMessage implements the ProcessMessage RPC
// This allows external clients (like Core/UI) to send messages to the agent
func (s *AgentServer) ProcessMessage(ctx context.Context, req *pb.ProcessMessageRequest) (*pb.ProcessMessageResponse, error) {
	if !s.isReady {
		return &pb.ProcessMessageResponse{
			Response:  "",
			Success:   false,
			Error:     fmt.Sprintf("Agent not ready: %s", s.readyReason),
			SessionId: req.SessionId,
		}, nil
	}

	// Process the message through the agent
	response, err := s.agent.Chat(ctx, req.Message)
	if err != nil {
		return &pb.ProcessMessageResponse{
			Response:  "",
			Success:   false,
			Error:     fmt.Sprintf("Failed to process message: %v", err),
			SessionId: req.SessionId,
		}, nil
	}

	// Track session if provided
	if req.SessionId != "" {
		s.trackSession(req.SessionId, req.Message)
	}

	return &pb.ProcessMessageResponse{
		Response:  response,
		Success:   true,
		Error:     "",
		SessionId: req.SessionId,
	}, nil
}

// GetAgentState implements the GetAgentState RPC
// This allows clients to check the health and readiness of the agent
func (s *AgentServer) GetAgentState(ctx context.Context, req *pb.GetAgentStateRequest) (*pb.GetAgentStateResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := "ready"
	if !s.isReady {
		status = "not_ready"
	}

	return &pb.GetAgentStateResponse{
		Ready:   s.isReady,
		Status:  status,
		Message: s.readyReason,
	}, nil
}

// trackSession records a session interaction
func (s *AgentServer) trackSession(sessionID string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		session = &SessionState{
			SessionID: sessionID,
			Created:   getCurrentTimestamp(),
			Messages:  []string{},
		}
		s.sessions[sessionID] = session
	}

	session.LastUsed = getCurrentTimestamp()
	session.Messages = append(session.Messages, message)
}

// SetReady sets the agent's readiness state
func (s *AgentServer) SetReady(ready bool, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.isReady = ready
	s.readyReason = reason
}

// GetSessions returns all tracked sessions (for debugging/monitoring)
func (s *AgentServer) GetSessions() map[string]*SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	sessionsCopy := make(map[string]*SessionState)
	for k, v := range s.sessions {
		sessionsCopy[k] = v
	}
	return sessionsCopy
}

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return int64(1) // Placeholder; use time.Now().Unix() when needed
}
