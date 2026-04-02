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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// loadTimezone loads the specified timezone location. If the timezone is empty
// or loading fails, it falls back to Asia/Hong_Kong (HKT, UTC+8).
func loadTimezone(timezone string) *time.Location {
	defaultLoc, _ := time.LoadLocation("Asia/Hong_Kong")
	if defaultLoc == nil {
		defaultLoc = time.FixedZone("HKT", 8*60*60)
	}
	if timezone == "" {
		return defaultLoc
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("failed to load timezone %q, defaulting to %q: %v", timezone, defaultLoc.String(), err)
		return defaultLoc
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

// parseTimeFlexible attempts to parse a time string using multiple formats.
// If the input ends with "Z", we first try interpreting the wall-clock value
// in the configured user timezone to avoid unintended UTC shifts from LLM output.
func parseTimeFlexible(timeStr string, loc *time.Location) (time.Time, error) {
	// 1. If the AI appended a 'Z' (UTC marker), strip it off so we can treat it as local time.
	cleanStr := strings.TrimSuffix(timeStr, "Z")

	// 2. Try parsing it as an ISO string but in the user's specific location (HKT)
	t, err := time.ParseInLocation("2006-01-02T15:04:05", cleanStr, loc)
	if err == nil {
		return t, nil
	}

	// 3. Try parsing it as the standard string but in the user's specific location
	t, err = time.ParseInLocation("2006-01-02 15:04:05", cleanStr, loc)
	if err == nil {
		return t, nil
	}

	// RFC3339 format: "YYYY-MM-DDTHH:MM:SSZ"
	t, err = time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t.In(loc), nil
	}

	// RFC3339Nano format: supports fractional seconds (e.g., "YYYY-MM-DDTHH:MM:SS.sssZ")
	t, err = time.Parse(time.RFC3339Nano, timeStr)
	if err == nil {
		return t.In(loc), nil
	}

	return time.Time{}, err
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
	if t.client == nil {
		return fmt.Sprintf("Error: " + "t.client is unexpectedly nil"), nil
	}
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
		return fmt.Sprintf("Error: " + "invalid create event payload: %v", err), nil
	}

	startTime, err := parseTimeFlexible(p.StartTime, t.loc)
	if err != nil {
		return fmt.Sprintf("Error: " + "invalid start_time format: %v", err), nil
	}
	endTime, err := parseTimeFlexible(p.EndTime, t.loc)
	if err != nil {
		return fmt.Sprintf("Error: " + "invalid end_time format: %v", err), nil
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
		logGRPCError("create_event", err)
		return fmt.Sprintf("Failed to create event: %v", err), nil
	}

	if resp == nil {
		return "Action is pending confirmation.", nil
	}
	if resp.Event == nil {
		return "Event proposed and pending user confirmation in the UI.", nil
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
	if t.client == nil {
		return fmt.Sprintf("Error: " + "t.client is unexpectedly nil"), nil
	}
	type payload struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return fmt.Sprintf("Error: " + "invalid get events payload: %v", err), nil
	}

	startTime, err := parseTimeFlexible(p.StartTime, t.loc)
	if err != nil {
		return fmt.Sprintf("Error: " + "invalid start_time format: %v", err), nil
	}
	endTime, err := parseTimeFlexible(p.EndTime, t.loc)
	if err != nil {
		return fmt.Sprintf("Error: " + "invalid end_time format: %v", err), nil
	}

	req := &pb.GetEventsRequest{
		UserId:    getUserID(ctx),
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
	}

	resp, err := t.client.GetEvents(ctx, req)
	if err != nil {
		logGRPCError("get_events", err)
		return fmt.Sprintf("Error: failed to get events in database: %v", err), nil
	}

	if resp == nil {
		return fmt.Sprintf("Error: " + "get_events returned nil response"), nil
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
			"start_time":  time.Unix(event.StartTime, 0).In(t.loc).Format("2006-01-02 15:04:05"),
			"end_time":    time.Unix(event.EndTime, 0).In(t.loc).Format("2006-01-02 15:04:05"),
			"location":    event.Location,
			"attendees":   event.Attendees,
		}
	}

	jsonBytes, err := json.Marshal(eventDetails)
	if err != nil {
		return fmt.Sprintf("Error: " + "failed to marshal event details: %v", err), nil
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
	return `Update an existing calendar event. Only use this when user explicitly asks to modify an existing event.
	You MUST provide the event id. Do NOT update by title alone; duplicate titles are allowed.
	Input should be a JSON object with fields:
        - id: string (required)
        - title: string (optional, leave null or empty to keep original)
        - description: string (optional, leave null or empty to keep original)
        - start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" or ISO8601 (optional, leave null or empty to keep original)
        - end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" or ISO8601 (optional, leave null or empty to keep original)
        - location: string (optional, leave null or empty to keep original)
        - attendees: array of email addresses (optional, leave empty to keep original)`
}

func (t *updateEventTool) Call(ctx context.Context, input string) (string, error) {
	if t.client == nil {
		return fmt.Sprintf("Error: " + "t.client is unexpectedly nil"), nil
	}
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
		return fmt.Sprintf("Error: " + "invalid update event payload: %v", err), nil
	}

	// If any field is empty, retrieve the original event to preserve existing data
	if p.Title == "" || p.Description == "" || p.Location == "" || p.StartTime == "" || p.EndTime == "" {
		reqGet := &pb.GetEventsRequest{
			UserId:    getUserID(ctx),
			StartTime: time.Now().Add(-365 * 24 * time.Hour).Unix(),
			EndTime:   time.Now().Add(365 * 24 * time.Hour).Unix(),
		}
		respGet, errGet := t.client.GetEvents(ctx, reqGet)
		if errGet == nil && respGet != nil {
			for _, ev := range respGet.Events {
				if ev.Id == p.ID {
					if p.Title == "" {
						p.Title = ev.Title
					}
					if p.Description == "" {
						p.Description = ev.Description
					}
					if p.Location == "" {
						p.Location = ev.Location
					}
					if p.StartTime == "" && ev.StartTime != 0 {
						p.StartTime = time.Unix(ev.StartTime, 0).Format(time.RFC3339)
					}
					if p.EndTime == "" && ev.EndTime != 0 {
						p.EndTime = time.Unix(ev.EndTime, 0).Format(time.RFC3339)
					}
					if len(p.Attendees) == 0 {
						p.Attendees = ev.Attendees
					}
					break
				}
			}
		}
	}

	if strings.TrimSpace(p.ID) == "" {
		return "Error: update_event requires a non-empty event id. Use search_events first to find the exact event.", nil
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
		startTime, err := parseTimeFlexible(p.StartTime, t.loc)
		if err != nil {
			return fmt.Sprintf("Error: " + "invalid start_time format: %v", err), nil
		}
		req.StartTime = startTime.Unix()
	}
	if p.EndTime != "" {
		endTime, err := parseTimeFlexible(p.EndTime, t.loc)
		if err != nil {
			return fmt.Sprintf("Error: " + "invalid end_time format: %v", err), nil
		}
		req.EndTime = endTime.Unix()
	}

	resp, err := t.client.UpdateEvent(ctx, req)
	if err != nil {
		logGRPCError("update_event", err)
		return fmt.Sprintf("Failed to update event: %v", err), nil
	}

	if resp == nil {
		return "Action is pending confirmation.", nil
	}
	if resp.Event == nil {
		return "Event proposed and pending user confirmation in the UI.", nil
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
	if t.client == nil {
		return fmt.Sprintf("Error: " + "t.client is unexpectedly nil"), nil
	}
	type payload struct {
		ID string `json:"id"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return fmt.Sprintf("Error: " + "invalid delete event payload: %v", err), nil
	}

	req := &pb.DeleteEventRequest{
		UserId: getUserID(ctx),
		Id:     p.ID,
	}

	resp, err := t.client.DeleteEvent(ctx, req)
	if err != nil {
		logGRPCError("delete_event", err)
		return fmt.Sprintf("Error: failed to delete event in database: %v", err), nil
	}

	if resp == nil {
		return fmt.Sprintf("Error: " + "delete_event returned nil response"), nil
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
	if t.client == nil {
		return fmt.Sprintf("Error: " + "t.client is unexpectedly nil"), nil
	}
	type payload struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		Duration  int64  `json:"duration"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return fmt.Sprintf("Error: " + "invalid get available slots payload: %v", err), nil
	}

	startTime, err := parseTimeFlexible(p.StartTime, t.loc)
	if err != nil {
		return fmt.Sprintf("Error: " + "invalid start_time format: %v", err), nil
	}
	endTime, err := parseTimeFlexible(p.EndTime, t.loc)
	if err != nil {
		return fmt.Sprintf("Error: " + "invalid end_time format: %v", err), nil
	}

	req := &pb.GetAvailableSlotsRequest{
		UserId:    getUserID(ctx),
		StartTime: startTime.Unix(),
		EndTime:   endTime.Unix(),
		Duration:  p.Duration * 60,
	}

	resp, err := t.client.GetAvailableSlots(ctx, req)
	if err != nil {
		logGRPCError("availability", err)
		return fmt.Sprintf("Error: failed to get available slots in database: %v", err), nil
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
	if t.client == nil {
		return fmt.Sprintf("Error: " + "t.client is unexpectedly nil"), nil
	}
	type payload struct {
		Query string `json:"query"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {

		return fmt.Sprintf("Error: " + "invalid search events payload: %v", err), nil
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
		logGRPCError("search_events", err)
		return fmt.Sprintf("Error: failed to search events in database: %v", err), nil
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
			"start_time":  time.Unix(event.StartTime, 0).In(loadTimezone("Asia/Hong_Kong")).Format("2006-01-02 15:04:05"),
			"end_time":    time.Unix(event.EndTime, 0).In(loadTimezone("Asia/Hong_Kong")).Format("2006-01-02 15:04:05"),
			"location":    event.Location,
			"attendees":   event.Attendees,
		}
	}

	jsonBytes, err := json.Marshal(eventDetails)
	if err != nil {
		return fmt.Sprintf("Error: " + "failed to marshal event details: %v", err), nil
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

// logGRPCError logs a detailed breakdown of a gRPC error including the status
// code and message so it's easy to trace calendar backend failures.
func logGRPCError(tool string, err error) {
	if err == nil {
		return
	}
	if st, ok := status.FromError(err); ok {
		log.Printf("[CalendarTool:%s] gRPC error — code=%s message=%q", tool, st.Code(), st.Message())
		if st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded {
			log.Printf("[CalendarTool:%s] HINT: Calendar backend appears to be unreachable or slow. Check that the calendar service is running and healthy.", tool)
		}
	} else {
		log.Printf("[CalendarTool:%s] non-gRPC error — %v", tool, err)
	}
}
