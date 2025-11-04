package grpcclient

import (
	"context"
	"fmt"

	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc"
)

// AgentClient is a client for the Orbi Agent's gRPC service
type AgentClient struct {
	conn   *grpc.ClientConn
	client pb.AgentServiceClient
}

// NewAgentClient creates a new client for the Agent service
func NewAgentClient(agentAddr string) (*AgentClient, error) {
	conn, err := grpc.Dial(agentAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent at %s: %w", agentAddr, err)
	}

	return &AgentClient{
		conn:   conn,
		client: pb.NewAgentServiceClient(conn),
	}, nil
}

// ProcessMessage sends a user message to the agent and gets a response
func (ac *AgentClient) ProcessMessage(ctx context.Context, message, sessionID string) (string, error) {
	req := &pb.ProcessMessageRequest{
		Message:   message,
		SessionId: sessionID,
	}

	resp, err := ac.client.ProcessMessage(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to process message: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("agent error: %s", resp.Error)
	}

	return resp.Response, nil
}

// GetAgentState checks the health and readiness of the agent
func (ac *AgentClient) GetAgentState(ctx context.Context, sessionID string) (*pb.GetAgentStateResponse, error) {
	req := &pb.GetAgentStateRequest{
		SessionId: sessionID,
	}

	resp, err := ac.client.GetAgentState(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent state: %w", err)
	}

	return resp, nil
}

// Close closes the connection to the agent
func (ac *AgentClient) Close() error {
	return ac.conn.Close()
}
