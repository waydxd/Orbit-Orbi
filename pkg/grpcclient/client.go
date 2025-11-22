package grpcclient

import (
	"context"
	"fmt"
	"time"

	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// CalendarClient wraps the gRPC calendar service client
type CalendarClient struct {
	conn        *grpc.ClientConn
	client      pb.CalendarServiceClient
	callTimeout time.Duration
}

// NewCalendarClient creates a new calendar client
func NewCalendarClient(address string) (*CalendarClient, error) {
	// Set up connection options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Establish connection
	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to calendar service: %w", err)
	}

	return &CalendarClient{
		conn:        conn,
		client:      pb.NewCalendarServiceClient(conn),
		callTimeout: 5 * time.Second,
	}, nil
}

// Close closes the gRPC connection
func (c *CalendarClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// withDeadlineAndMeta ensures per-RPC deadline and forwards user/session metadata
func (c *CalendarClient) withDeadlineAndMeta(ctx context.Context) (context.Context, context.CancelFunc) {
	// Apply timeout if not already set
	if _, ok := ctx.Deadline(); !ok {
		ctx, _ = context.WithTimeout(ctx, c.callTimeout)
	}

	// Forward incoming metadata to outgoing context
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx, func() {}
}

// CreateEvent creates a new calendar event
func (c *CalendarClient) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.CreateEventResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.CreateEvent(ctx, req)
}

// GetEvents retrieves events within a time range
func (c *CalendarClient) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.GetEvents(ctx, req)
}

// UpdateEvent updates an existing event
func (c *CalendarClient) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.UpdateEventResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.UpdateEvent(ctx, req)
}

// DeleteEvent deletes an event
func (c *CalendarClient) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.DeleteEvent(ctx, req)
}

// GetAvailableSlots finds available time slots
func (c *CalendarClient) GetAvailableSlots(ctx context.Context, req *pb.GetAvailableSlotsRequest) (*pb.GetAvailableSlotsResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.GetAvailableSlots(ctx, req)
}

// CalendarRetrievalTool implements orbi.RetrievalTool using CalendarClient.
// It is defined in the orbi package to avoid import cycles; this file only
// provides the low-level gRPC client.
