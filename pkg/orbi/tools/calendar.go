package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tmc/langchaingo/tools"
	"github.com/waydxd/Orbit-Orbi/pkg/grpcclient"
	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc/metadata"
)

// loadTimezone loads the specified timezone location. If the timezone is empty
// or loading fails, it returns UTC as a fallback.
func loadTimezone(timezone string) *time.Location {
	if timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("failed to load timezone %q, defaulting to UTC: %v", timezone, err)
		return time.UTC
	}
	return loc
}

// NewCalendarTools creates a new set of calendar tools.
func NewCalendarTools(calendarServiceAddr string, timezone string) ([]tools.Tool, error) {
	calendarClient, err := grpcclient.NewCalendarClient(calendarServiceAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create calendar client: %w", err)
	}

	loc := loadTimezone(timezone)

	return []tools.Tool{
		&createEventTool{client: calendarClient, loc: loc},
		&getEventsTool{client: calendarClient, loc: loc},
		&updateEventTool{client: calendarClient, loc: loc},
		&deleteEventTool{client: calendarClient},
		&getAvailableSlotsTool{client: calendarClient, loc: loc},
		&searchEventsTool{client: calendarClient},
	}, nil
}

// createEventTool wraps the CreateEvent gRPC call as a langchain tool
type createEventTool struct {
	client *grpcclient.CalendarClient
	loc    *time.Location
}

func (t *createEventTool) Name() string { return "create_event" }

func (t *createEventTool) Description() string {
	return `Create a new calendar event. Input should be a JSON object with fields:
	- title: string (required)
	- description: string (optional)
	- start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required, e.g., "2025-11-23 09:00:00")
	- end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required, e.g., "2025-11-23 10:00:00")
	- location: string (optional)
	- attendees: array of email addresses (optional)`
}

func (t *createEventTool) Call(ctx context.Context, input string) (string, error) {
	type payload struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		StartTime   string   `json:"start_time"`
		EndTime     string   `json:"end_time"`
		Location    string   `json:"location"`
		Attendees   []string `json:"attendees"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid create event payload: %w", err)
	}

	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.StartTime, t.loc)
	if err != nil {
		return "", fmt.Errorf("invalid start_time format: %w", err)
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.EndTime, t.loc)
	if err != nil {
		return "", fmt.Errorf("invalid end_time format: %w", err)
	}

	req := &pb.CreateEventRequest{
		UserId:      getUserID(ctx),
		Title:       p.Title,
		Description: p.Description,
		StartTime:   startTime.Unix(),
		EndTime:     endTime.Unix(),
		Location:    p.Location,
		Attendees:   p.Attendees,
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
	loc    *time.Location
}

func (t *getEventsTool) Name() string { return "get_events" }

func (t *getEventsTool) Description() string {
	return `Get calendar events within a time range. Input should be a JSON object with fields:
	- start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required)
	- end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required)`
}

func (t *getEventsTool) Call(ctx context.Context, input string) (string, error) {
	type payload struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid get events payload: %w", err)
	}

	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.StartTime, t.loc)
	if err != nil {
		return "", fmt.Errorf("invalid start_time format: %w", err)
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.EndTime, t.loc)
	if err != nil {
		return "", fmt.Errorf("invalid end_time format: %w", err)
	}

	req := &pb.GetEventsRequest{
		UserId:    getUserID(ctx),
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
	}

	resp, err := t.client.GetEvents(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get events: %w", err)
	}

	if len(resp.Events) == 0 {
		return "No events found", nil
	}

	eventDetails := make([]map[string]interface{}, len(resp.Events))
	for i, event := range resp.Events {
		eventDetails[i] = map[string]interface{}{
			"id":          event.Id,
			"title":       event.Title,
			"description": event.Description,
			"start_time":  time.Unix(event.StartTime, 0).Format(time.RFC3339),
			"end_time":    time.Unix(event.EndTime, 0).Format(time.RFC3339),
			"location":    event.Location,
			"attendees":   event.Attendees,
		}
	}

	jsonBytes, err := json.Marshal(eventDetails)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event details: %w", err)
	}

	return string(jsonBytes), nil
}

// updateEventTool wraps the UpdateEvent gRPC call as a langchain tool
type updateEventTool struct {
	client *grpcclient.CalendarClient
	loc    *time.Location
}

func (t *updateEventTool) Name() string { return "update_event" }

func (t *updateEventTool) Description() string {
	return `Update an existing calendar event. You MUST know the event ID to use this tool. If you don't know the event ID, use the "search_events" tool first. Input should be a JSON object with fields:
	- id: string (required)
	- title: string (optional)
	- description: string (optional)
	- start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (optional)
	- end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (optional)
	- location: string (optional)
	- attendees: array of email addresses (optional)`
}

func (t *updateEventTool) Call(ctx context.Context, input string) (string, error) {
	type payload struct {
		ID          string   `json:"id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		StartTime   string   `json:"start_time"`
		EndTime     string   `json:"end_time"`
		Location    string   `json:"location"`
		Attendees   []string `json:"attendees"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid update event payload: %w", err)
	}

	req := &pb.UpdateEventRequest{
		UserId:      getUserID(ctx),
		Id:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		Location:    p.Location,
		Attendees:   p.Attendees,
	}

	if p.StartTime != "" {
		startTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.StartTime, t.loc)
		if err != nil {
			return "", fmt.Errorf("invalid start_time format: %w", err)
		}
		req.StartTime = startTime.Unix()
	}
	if p.EndTime != "" {
		endTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.EndTime, t.loc)
		if err != nil {
			return "", fmt.Errorf("invalid end_time format: %w", err)
		}
		req.EndTime = endTime.Unix()
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

func (t *deleteEventTool) Name() string { return "delete_event" }

func (t *deleteEventTool) Description() string {
	return `Delete a calendar event. Input should be a JSON object with field:
	- id: string (required)`
}

func (t *deleteEventTool) Call(ctx context.Context, input string) (string, error) {
	type payload struct {
		ID string `json:"id"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid delete event payload: %w", err)
	}

	req := &pb.DeleteEventRequest{
		UserId: getUserID(ctx),
		Id:     p.ID,
	}

	resp, err := t.client.DeleteEvent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to delete event: %w", err)
	}

	if resp.Success {
		return "Event deleted successfully", nil
	}
	return "Failed to delete event", nil
}

// getAvailableSlotsTool wraps the GetAvailableSlots gRPC call as a langchain tool
type getAvailableSlotsTool struct {
	client *grpcclient.CalendarClient
	loc    *time.Location
}

func (t *getAvailableSlotsTool) Name() string { return "availability" }

func (t *getAvailableSlotsTool) Description() string {
	return `Find available time slots in the calendar. Input should be a JSON object with fields:
	- start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required)
	- end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required)
	- duration: duration in minutes (required)`
}

func (t *getAvailableSlotsTool) Call(ctx context.Context, input string) (string, error) {
	type payload struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Duration  int64  `json:"duration"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid get available slots payload: %w", err)
	}

	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.StartTime, t.loc)
	if err != nil {
		return "", fmt.Errorf("invalid start_time format: %w", err)
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", p.EndTime, t.loc)
	if err != nil {
		return "", fmt.Errorf("invalid end_time format: %w", err)
	}

	req := &pb.GetAvailableSlotsRequest{
		UserId:    getUserID(ctx),
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
		Duration:  p.Duration * 60,
	}

	resp, err := t.client.GetAvailableSlots(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get available slots: %w", err)
	}

	return fmt.Sprintf("Found %d available slots", len(resp.Slots)), nil
}

// searchEventsTool wraps the GetEvents gRPC call as a langchain tool
type searchEventsTool struct {
	client *grpcclient.CalendarClient
}

func (t *searchEventsTool) Name() string { return "search_events" }

func (t *searchEventsTool) Description() string {
	return `Search for calendar events by a query string. This is useful for finding a specific event to get its ID. The search is case-insensitive and matches partial strings in title, description, and location. Input should be a JSON object with the field:
	- query: string (required)`
}

func (t *searchEventsTool) Call(ctx context.Context, input string) (string, error) {
	type payload struct {
		Query string `json:"query"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid search events payload: %w", err)
	}

	// For now, we'll just use the get_events tool with a wide time range
	// to search for events.
	req := &pb.GetEventsRequest{
		UserId:    getUserID(ctx),
		StartTime: time.Now().Add(-365 * 24 * time.Hour).Unix(),
		EndTime:   time.Now().Add(365 * 24 * time.Hour).Unix(),
	}

	resp, err := t.client.GetEvents(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get events: %w", err)
	}

	queryLower := strings.ToLower(p.Query)
	var matchedEvents []*pb.Event
	for _, event := range resp.Events {
		// Case-insensitive substring matching on title, description, and location
		if strings.Contains(strings.ToLower(event.Title), queryLower) ||
			strings.Contains(strings.ToLower(event.Description), queryLower) ||
			strings.Contains(strings.ToLower(event.Location), queryLower) {
			matchedEvents = append(matchedEvents, event)
		}
	}

	if len(matchedEvents) == 0 {
		return "No events found", nil
	}

	eventDetails := make([]map[string]interface{}, len(matchedEvents))
	for i, event := range matchedEvents {
		eventDetails[i] = map[string]interface{}{
			"id":          event.Id,
			"title":       event.Title,
			"description": event.Description,
			"start_time":  time.Unix(event.StartTime, 0).Format(time.RFC3339),
			"end_time":    time.Unix(event.EndTime, 0).Format(time.RFC3339),
			"location":    event.Location,
			"attendees":   event.Attendees,
		}
	}

	jsonBytes, err := json.Marshal(eventDetails)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event details: %w", err)
	}

	return string(jsonBytes), nil
}

func getUserID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		vals := md.Get("user_id")
		if len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
