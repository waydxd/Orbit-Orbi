package orbi

import (
	"context"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"
	"github.com/waydxd/Orbit-Orbi/pkg/grpcclient"
	pb "github.com/waydxd/Orbit-Orbi/proto"
)

// Agent represents the Orbi chatbot agent
type Agent struct {
	calendarClient *grpcclient.CalendarClient
	llm            llms.Model
	executor       *agents.Executor
}

// Config holds the configuration for the Orbi agent
type Config struct {
	CalendarServiceAddr string
	OpenAIAPIKey        string
	Model               string
}

// NewAgent creates a new Orbi agent
func NewAgent(cfg Config) (*Agent, error) {
	// Create calendar client
	calendarClient, err := grpcclient.NewCalendarClient(cfg.CalendarServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar client: %w", err)
	}

	// Initialize LLM
	llm, err := openai.New(
		openai.WithToken(cfg.OpenAIAPIKey),
		openai.WithModel(cfg.Model),
	)
	if err != nil {
		calendarClient.Close()
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	agent := &Agent{
		calendarClient: calendarClient,
		llm:            llm,
	}

	// Create tools for the agent
	calendarTools := agent.createCalendarTools()

	// Create agent executor
	executor := agents.NewExecutor(
		agents.NewConversationalAgent(llm, calendarTools),
		agents.WithMaxIterations(5),
	)

	agent.executor = executor

	return agent, nil
}

// Close closes the agent and its resources
func (a *Agent) Close() error {
	return a.calendarClient.Close()
}

// Chat processes a user message and returns a response
func (a *Agent) Chat(ctx context.Context, message string) (string, error) {
	result, err := chains.Run(ctx, a.executor, message)
	if err != nil {
		return "", fmt.Errorf("failed to process message: %w", err)
	}
	return result, nil
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

func (t *createEventTool) Name() string {
	return "create_calendar_event"
}

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
	// In a real implementation, parse the JSON input and create the request
	// For this template, we'll return a placeholder
	req := &pb.CreateEventRequest{
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

func (t *getEventsTool) Name() string {
	return "get_calendar_events"
}

func (t *getEventsTool) Description() string {
	return `Get calendar events within a time range. Input should be a JSON object with fields:
	- start_time: Unix timestamp in seconds (required)
	- end_time: Unix timestamp in seconds (required)
	- status: string (optional, e.g., "confirmed", "tentative", "cancelled")`
}

func (t *getEventsTool) Call(ctx context.Context, input string) (string, error) {
	// In a real implementation, parse the JSON input and create the request
	req := &pb.GetEventsRequest{
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

func (t *updateEventTool) Name() string {
	return "update_calendar_event"
}

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
	// In a real implementation, parse the JSON input and create the request
	req := &pb.UpdateEventRequest{
		Id:    "sample-id",
		Title: "Updated Event",
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

func (t *deleteEventTool) Name() string {
	return "delete_calendar_event"
}

func (t *deleteEventTool) Description() string {
	return `Delete a calendar event. Input should be a JSON object with field:
	- id: string (required)`
}

func (t *deleteEventTool) Call(ctx context.Context, input string) (string, error) {
	// In a real implementation, parse the JSON input and create the request
	req := &pb.DeleteEventRequest{
		Id: "sample-id",
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

func (t *getAvailableSlotsTool) Name() string {
	return "get_available_slots"
}

func (t *getAvailableSlotsTool) Description() string {
	return `Find available time slots in the calendar. Input should be a JSON object with fields:
	- start_time: Unix timestamp in seconds (required)
	- end_time: Unix timestamp in seconds (required)
	- duration: duration in seconds (required)`
}

func (t *getAvailableSlotsTool) Call(ctx context.Context, input string) (string, error) {
	// In a real implementation, parse the JSON input and create the request
	req := &pb.GetAvailableSlotsRequest{
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
