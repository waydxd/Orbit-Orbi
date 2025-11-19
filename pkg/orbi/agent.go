package orbi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
	"github.com/waydxd/Orbit-Orbi/pkg/grpcclient"
	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc/metadata"
)

// Agent represents the Orbi chatbot agent
type Agent struct {
	cfg            Config
	calendarClient *grpcclient.CalendarClient // may be nil until initialized
	llm            llms.Model                 // may be nil until initialized
	executor       *agents.Executor

	mu          sync.Mutex // protects the following
	initialized bool
}

// Config holds the configuration for the Orbi agent
type Config struct {
	CalendarServiceAddr string
	OpenAIAPIKey        string
	Model               string
	BaseURL             string
}

// NewAgent creates a new Orbi agent but does NOT initialize heavy resources
// such as the calendar gRPC client or the LLM. Those are created lazily on
// first use (for example, when handling a core connection or processing the
// first message). This makes the agent lightweight at startup and allows it
// to accept incoming core connections without pre-allocating per-core
// resources.
func NewAgent(cfg Config) (*Agent, error) {
	agent := &Agent{
		cfg: cfg,
	}
	// Do not initialize external clients here. Return quickly so the caller
	// can start the gRPC server and accept core connections. Resources will
	// be created lazily in ensureInitialized.
	return agent, nil
}

// ensureInitialized performs one-time initialization of heavy resources.
// It's safe to call concurrently; only one goroutine will run the init
// sequence and others will block until it completes.
func (a *Agent) ensureInitialized(ctx context.Context) error {
	// Fast path without locking
	if a.isInitialized() {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Double-check
	if a.initialized {
		return nil
	}

	// Create calendar client
	calendarClient, err := grpcclient.NewCalendarClient(a.cfg.CalendarServiceAddr)
	if err != nil {
		return fmt.Errorf("failed to create calendar client: %w", err)
	}

	// Initialize LLM
	llmModel, err := openai.New(
		openai.WithBaseURL(a.cfg.BaseURL),
		openai.WithToken(a.cfg.OpenAIAPIKey),
		openai.WithModel(a.cfg.Model),
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithEmbeddingModel(a.cfg.Model),
	)
	if err != nil {
		_ = calendarClient.Close()
		return fmt.Errorf("failed to initialize LLM: %w", err)
	}

	// Assign resources to receiver
	a.calendarClient = calendarClient
	a.llm = llmModel

	// Create tools now that calendarClient is available
	calendarTools := a.createCalendarTools()

	// Create agent executor
	executor := agents.NewExecutor(
		agents.NewConversationalAgent(a.llm, calendarTools),
		agents.WithMaxIterations(5),
	)
	a.executor = executor

	a.initialized = true
	return nil
}

// helper to check initialization without lock
func (a *Agent) isInitialized() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.initialized
}

// Close closes the agent and its resources if they were initialized
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var firstErr error
	if a.calendarClient != nil {
		if err := a.calendarClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// If LLM implements a Close (it doesn't in this code), close it here.
	// For now, just clear references.
	a.calendarClient = nil
	a.llm = nil
	a.executor = nil
	a.initialized = false

	return firstErr
}

// Chat processes a user message and returns a response. It will lazily
// initialize internal resources on first use.
func (a *Agent) Chat(ctx context.Context, message string) (string, error) {
	// Ensure the heavy resources are initialized
	if err := a.ensureInitialized(ctx); err != nil {
		return "", err
	}

	result, err := chains.Run(ctx, a.executor, message)
	if err != nil {
		return "", fmt.Errorf("failed to process message: %w", err)
	}
	return result, nil
}

// getUserID extracts user_id from metadata if present
func getUserID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		vals := md.Get("user_id")
		if len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// createCalendarTools creates tools for calendar operations
func (a *Agent) createCalendarTools() []tools.Tool {
	return []tools.Tool{
		&createEventTool{client: a.calendarClient},
		&getEventsTool{client: a.calendarClient},
		&updateEventTool{client: a.calendarClient},
		&deleteEventTool{client: a.calendarClient},
		&getAvailableSlotsTool{client: a.calendarClient},
	}
}

// createEventTool wraps the CreateEvent gRPC call as a langchain tool
type createEventTool struct {
	client *grpcclient.CalendarClient
}

func (t *createEventTool) Name() string { return "create_calendar_event" }

func (t *createEventTool) Description() string {
	return `Create a new calendar event. Input should be a JSON object with fields:
	- title: string (required)
	- description: string (optional)
	- start_time: Unix timestamp in seconds (required)
	- end_time: Unix timestamp in seconds (required)
	- location: string (optional)
	- attendees: array of email addresses (optional)`
}

func (t *createEventTool) Call(ctx context.Context, input string) (string, error) {
	// Placeholder parsing
	req := &pb.CreateEventRequest{
		UserId:      getUserID(ctx),
		Title:       "Sample Event",
		Description: "Created via Orbi agent",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(1 * time.Hour).Unix(),
	}

	resp, err := t.client.CreateEvent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create event: %w", err)
	}

	return fmt.Sprintf("Event created: %s (ID: %s)", resp.Event.Title, resp.Event.Id), nil
}

// getEventsTool wraps the GetEvents gRPC call as a langchain tool
type getEventsTool struct {
	client *grpcclient.CalendarClient
}

func (t *getEventsTool) Name() string { return "get_calendar_events" }

func (t *getEventsTool) Description() string {
	return `Get calendar events within a time range. Input should be a JSON object with fields:
	- start_time: Unix timestamp in seconds (required)
	- end_time: Unix timestamp in seconds (required)
	- status: string (optional, e.g., "confirmed", "tentative", "cancelled")`
}

func (t *getEventsTool) Call(ctx context.Context, input string) (string, error) {
	req := &pb.GetEventsRequest{
		UserId:    getUserID(ctx),
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Add(24 * time.Hour).Unix(),
	}

	resp, err := t.client.GetEvents(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get events: %w", err)
	}

	return fmt.Sprintf("Found %d events", len(resp.Events)), nil
}

// updateEventTool wraps the UpdateEvent gRPC call as a langchain tool
type updateEventTool struct {
	client *grpcclient.CalendarClient
}

func (t *updateEventTool) Name() string { return "update_calendar_event" }

func (t *updateEventTool) Description() string {
	return `Update an existing calendar event. Input should be a JSON object with fields:
	- id: string (required)
	- title: string (optional)
	- description: string (optional)
	- start_time: Unix timestamp in seconds (optional)
	- end_time: Unix timestamp in seconds (optional)
	- location: string (optional)
	- attendees: array of email addresses (optional)
	- status: string (optional)`
}

func (t *updateEventTool) Call(ctx context.Context, input string) (string, error) {
	req := &pb.UpdateEventRequest{
		UserId: getUserID(ctx),
		Id:     "sample-id",
		Title:  "Updated Event",
	}

	resp, err := t.client.UpdateEvent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to update event: %w", err)
	}

	return fmt.Sprintf("Event updated: %s", resp.Event.Title), nil
}

// deleteEventTool wraps the DeleteEvent gRPC call as a langchain tool
type deleteEventTool struct {
	client *grpcclient.CalendarClient
}

func (t *deleteEventTool) Name() string { return "delete_calendar_event" }

func (t *deleteEventTool) Description() string {
	return `Delete a calendar event. Input should be a JSON object with field:
	- id: string (required)`
}

func (t *deleteEventTool) Call(ctx context.Context, input string) (string, error) {
	req := &pb.DeleteEventRequest{
		UserId: getUserID(ctx),
		Id:     "sample-id",
	}

	resp, err := t.client.DeleteEvent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to delete event: %w", err)
	}

	if resp.Success {
		return "Event deleted successfully", nil
	}
	return fmt.Sprintf("Failed to delete event: %s", resp.Message), nil
}

// getAvailableSlotsTool wraps the GetAvailableSlots gRPC call as a langchain tool
type getAvailableSlotsTool struct {
	client *grpcclient.CalendarClient
}

func (t *getAvailableSlotsTool) Name() string { return "get_available_slots" }

func (t *getAvailableSlotsTool) Description() string {
	return `Find available time slots in the calendar. Input should be a JSON object with fields:
	- start_time: Unix timestamp in seconds (required)
	- end_time: Unix timestamp in seconds (required)
	- duration: duration in seconds (required)`
}

func (t *getAvailableSlotsTool) Call(ctx context.Context, input string) (string, error) {
	req := &pb.GetAvailableSlotsRequest{
		UserId:    getUserID(ctx),
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Add(7 * 24 * time.Hour).Unix(),
		Duration:  3600, // 1 hour
	}

	resp, err := t.client.GetAvailableSlots(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get available slots: %w", err)
	}

	return fmt.Sprintf("Found %d available slots", len(resp.Slots)), nil
}
