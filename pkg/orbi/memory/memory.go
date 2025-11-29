package memory

import (
	"context"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/types"
)

// AgentMemory interface for conversation state management
type AgentMemory interface {
	// SaveMessage adds a message to history
	SaveMessage(ctx context.Context, sessionID string, msg types.Message) error

	// GetMessages retrieves conversation history
	GetMessages(ctx context.Context, sessionID string, limit int) ([]types.Message, error)

	// ClearSession removes a session
	ClearSession(ctx context.Context, sessionID string) error
}

// InMemoryAgentMemory is a simple in-memory implementation of AgentMemory
type InMemoryAgentMemory struct {
	sessions map[string][]types.Message
}

// NewInMemoryAgentMemory creates a new in-memory memory store
func NewInMemoryAgentMemory() *InMemoryAgentMemory {
	return &InMemoryAgentMemory{
		sessions: make(map[string][]types.Message),
	}
}

// SaveMessage adds a message to history
func (m *InMemoryAgentMemory) SaveMessage(ctx context.Context, sessionID string, msg types.Message) error {
	m.sessions[sessionID] = append(m.sessions[sessionID], msg)
	return nil
}

// GetMessages retrieves conversation history
func (m *InMemoryAgentMemory) GetMessages(ctx context.Context, sessionID string, limit int) ([]types.Message, error) {
	messages, ok := m.sessions[sessionID]
	if !ok {
		return []types.Message{}, nil
	}

	if limit <= 0 || limit > len(messages) {
		limit = len(messages)
	}

	return messages[len(messages)-limit:], nil
}

// ClearSession removes a session
func (m *InMemoryAgentMemory) ClearSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}
