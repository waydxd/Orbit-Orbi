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

// CalendarDataClient wraps the Core's CalendarDataService client for read-only operations
type CalendarDataClient struct {
	conn        *grpc.ClientConn
	client      pb.CalendarDataServiceClient
	callTimeout time.Duration
}

// NewCalendarDataClient dials the Core's CalendarDataService endpoint
func NewCalendarDataClient(address string) (*CalendarDataClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to calendar data service: %w", err)
	}

	return &CalendarDataClient{
		conn:        conn,
		client:      pb.NewCalendarDataServiceClient(conn),
		callTimeout: 3 * time.Second,
	}, nil
}

// Close closes the connection
func (c *CalendarDataClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// withDeadlineAndMeta ensures per-RPC deadline and forwards incoming metadata
func (c *CalendarDataClient) withDeadlineAndMeta(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		ctx, _ = context.WithTimeout(ctx, c.callTimeout)
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx, func() {}
}

// GetCalendarData fetches events for a user in a time range
func (c *CalendarDataClient) GetCalendarData(ctx context.Context, req *pb.GetCalendarDataRequest) (*pb.GetCalendarDataResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.GetCalendarData(ctx, req)
}

// GetUserAvailability checks availability for a time slot
func (c *CalendarDataClient) GetUserAvailability(ctx context.Context, req *pb.GetUserAvailabilityRequest) (*pb.GetUserAvailabilityResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.GetUserAvailability(ctx, req)
}

// QueryEvents performs a query over events
func (c *CalendarDataClient) QueryEvents(ctx context.Context, req *pb.QueryEventsRequest) (*pb.QueryEventsResponse, error) {
	ctx, cancel := c.withDeadlineAndMeta(ctx)
	defer cancel()
	return c.client.QueryEvents(ctx, req)
}
