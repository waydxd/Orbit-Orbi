package grpcclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentClient is a client for the Orbi Agent's gRPC service
type AgentClient struct {
	conn   *grpc.ClientConn
	client pb.AgentServiceClient
}

// NewAgentClient creates a new client for the Agent service
func NewAgentClient(agentAddr string) (*AgentClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, agentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent at %s: %w", agentAddr, err)
	}

	return &AgentClient{
		conn:   conn,
		client: pb.NewAgentServiceClient(conn),
	}, nil
}

// ProcessMessage sends a user message to the agent and gets a response
func (ac *AgentClient) ProcessMessage(ctx context.Context, userID, message, sessionID string) (string, error) {
	req := &pb.ProcessMessageRequest{
		UserId:    userID,
		Message:   message,
		SessionId: sessionID,
	}

	// apply a reasonable deadline if not set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
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
func (ac *AgentClient) GetAgentState(ctx context.Context, userID, sessionID string) (*pb.GetAgentStateResponse, error) {
	req := &pb.GetAgentStateRequest{
		UserId:    userID,
		SessionId: sessionID,
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
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
