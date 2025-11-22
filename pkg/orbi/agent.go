package orbi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	// use ctx to avoid unused parameter warnings in static analysis
	_ = ctx
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

	if a.calendarClient != nil {
		if err := a.calendarClient.Close(); err != nil {
			return err
		}
	}

	// If LLM implements a Close (it doesn't in this code), close it here.
	// For now, just clear references.
	a.calendarClient = nil
	a.llm = nil
	a.executor = nil
	a.initialized = false

	return nil
}

// Chat processes a user message and returns a response. It will lazily
// initialize internal resources on first use.
func (a *Agent) Chat(ctx context.Context, message string) (string, error) {
	// Ensure the heavy resources are initialized
	if err := a.ensureInitialized(ctx); err != nil {
		return "", err
	}

	// Provide the LLM with the current runtime datetime so it doesn't rely on
	// its training-time knowledge when interpreting relative dates/times.
	// We include this as an explicit instruction prefix to the user's message.
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		log.Printf("failed to load timezone, defaulting to UTC: %v", err)
		loc = time.UTC
	}
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("2006-01-02 15:04:05")
	timezoneName := loc.String()

	augmented := fmt.Sprintf(`Current Date and Time: %s (Timezone: %s)

CRITICAL Instructions for handling dates and times:
1. Use the current date/time above as your ONLY reference for interpreting relative dates
2. When using calendar tools, provide datetime values in this EXACT format: "YYYY-MM-DD HH:MM:SS"

IMPORTANT: Default Action Behavior
- If the user provides event details (title, time, date) WITHOUT explicitly stating an action (like "show", "list", "find", "check"), you should DEFAULT to CREATING the event
- Examples that should CREATE events:
  * "Meeting tomorrow at 9am"
  * "Dentist appointment next Tuesday 2pm"
  * "Lunch with John on Friday at noon"
- Examples that should NOT create (query instead):
  * "What do I have tomorrow?"
  * "Show me my schedule for next week"
  * "Do I have any meetings on Monday?"
  * "Find available time tomorrow"

Datetime Format Examples (assuming current time is %s):
- For "tomorrow at 9am": use "2025-11-23 09:00:00"
- For "today at 2:30pm": use "2025-11-22 14:30:00"
- For "next Monday at 10am": calculate the date of next Monday and use "YYYY-MM-DD 10:00:00"

Format Rules:
- Use 24-hour format (00:00 to 23:59)
- Always include leading zeros (e.g., "09:00:00" not "9:0:0")
- All times are in %s timezone
- Do NOT calculate Unix timestamps - the system handles conversion

Example tool call for creating an event:
{
  "title": "Meeting",
  "start_time": "2025-11-23 09:00:00",
  "end_time": "2025-11-23 10:00:00"
}

User: %s`, currentTimeStr, timezoneName, currentTimeStr, timezoneName, message)

	result, err := chains.Run(ctx, a.executor, augmented)
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
	- start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required, e.g., "2025-11-23 09:00:00")
	- end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (required, e.g., "2025-11-23 10:00:00")
	- location: string (optional)
	- attendees: array of email addresses (optional)`
}

func (t *createEventTool) Call(ctx context.Context, input string) (string, error) {
	// Parse JSON input from the LLM/tool call
	type payload struct {
		Title       string      `json:"title"`
		Description string      `json:"description"`
		StartTime   interface{} `json:"start_time"` // Can be string or int64
		EndTime     interface{} `json:"end_time"`   // Can be string or int64
		Location    string      `json:"location"`
		Attendees   []string    `json:"attendees"`
		Recurrence  string      `json:"recurrence"`
		Status      string      `json:"status"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid create event payload: %w", err)
	}

	// If title is empty, try to extract it from the input or use a default
	if p.Title == "" {
		p.Title = "Event"
	}

	// Load Hong Kong timezone for parsing
	loc, _ := time.LoadLocation("Asia/Hong_Kong")
	if loc == nil {
		loc = time.UTC
	}

	// Parse start time (can be datetime string or Unix timestamp)
	var startTS int64
	var endTS int64

	if p.StartTime != nil {
		switch v := p.StartTime.(type) {
		case string:
			// Parse datetime string
			t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				// Try alternative formats
				t, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return "", fmt.Errorf("invalid start_time format: received '%s', expected 'YYYY-MM-DD HH:MM:SS' (e.g., '2025-11-23 09:00:00')", v)
				}
				t = t.In(loc)
			}
			startTS = t.Unix()
		case float64:
			startTS = int64(v)
		default:
			return "", fmt.Errorf("start_time must be a datetime string in format 'YYYY-MM-DD HH:MM:SS'")
		}
	}

	if p.EndTime != nil {
		switch v := p.EndTime.(type) {
		case string:
			// Parse datetime string
			t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				// Try alternative formats
				t, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return "", fmt.Errorf("invalid end_time format: received '%s', expected 'YYYY-MM-DD HH:MM:SS' (e.g., '2025-11-23 10:00:00')", v)
				}
				t = t.In(loc)
			}
			endTS = t.Unix()
		case float64:
			endTS = int64(v)
		default:
			return "", fmt.Errorf("end_time must be a datetime string in format 'YYYY-MM-DD HH:MM:SS'")
		}
	}

	// If times not provided, default to 1-hour event starting now
	if startTS == 0 || endTS == 0 {
		start := time.Now().In(loc)
		startTS = start.Unix()
		// Default to 1 hour duration
		endTS = start.Add(time.Hour).Unix()
	} else if endTS == 0 {
		// If only start time provided, default to 1 hour duration
		endTS = startTS + 3600
	} else if startTS == 0 {
		// If only end time provided, default start to 1 hour before
		startTS = endTS - 3600
	}

	// Adjust years if timestamp is in past year
	startTS = adjustTimestampYear(startTS)
	endTS = adjustTimestampYear(endTS)

	// Debug: log parsed timestamps
	user := getUserID(ctx)
	log.Printf("[agent.debug] createEvent start=%d (%s) end=%d (%s) user=%s",
		startTS, time.Unix(startTS, 0).In(loc).Format(time.RFC3339),
		endTS, time.Unix(endTS, 0).In(loc).Format(time.RFC3339),
		user,
	)

	// Validate the event falls within [now, now+1year]
	if err := validateEventWithinOneYear(startTS, endTS); err != nil {
		return "", fmt.Errorf("event time outside allowed window: %w", err)
	}

	req := &pb.CreateEventRequest{
		UserId:      getUserID(ctx),
		Title:       p.Title,
		Description: p.Description,
		StartTime:   startTS,
		EndTime:     endTS,
		Location:    p.Location,
		Attendees:   p.Attendees,
		Recurrence:  p.Recurrence,
		Status:      p.Status,
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
	// Parse JSON input for time range and optional status filter
	type payload struct {
		StartTime int64  `json:"start_time"`
		EndTime   int64  `json:"end_time"`
		Status    string `json:"status"`
	}
	var p payload
	if input != "" {
		_ = json.Unmarshal([]byte(input), &p)
	}

	// If caller provided timestamps, nudge years; otherwise leave zeros so
	// clampRangeToOneYearWindow will provide reasonable defaults.
	if p.StartTime != 0 {
		p.StartTime = adjustTimestampYear(p.StartTime)
	}
	if p.EndTime != 0 {
		p.EndTime = adjustTimestampYear(p.EndTime)
	}

	// Clamp requested range to allowed window
	clampedStart, clampedEnd := clampRangeToOneYearWindow(p.StartTime, p.EndTime)

	req := &pb.GetEventsRequest{
		UserId:    getUserID(ctx),
		StartTime: clampedStart,
		EndTime:   clampedEnd,
		Status:    p.Status,
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
	- start_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (optional)
	- end_time: datetime string in format "YYYY-MM-DD HH:MM:SS" (optional)
	- location: string (optional)
	- attendees: array of email addresses (optional)
	- status: string (optional)`
}

func (t *updateEventTool) Call(ctx context.Context, input string) (string, error) {
	// Parse JSON input for updates
	type payload struct {
		ID          string      `json:"id"`
		Title       string      `json:"title"`
		Description string      `json:"description"`
		StartTime   interface{} `json:"start_time"` // Can be string or int64
		EndTime     interface{} `json:"end_time"`   // Can be string or int64
		Location    string      `json:"location"`
		Attendees   []string    `json:"attendees"`
		Status      string      `json:"status"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid update event payload: %w", err)
	}
	if p.ID == "" {
		return "", fmt.Errorf("missing event id")
	}

	// Load Hong Kong timezone for parsing
	loc, _ := time.LoadLocation("Asia/Hong_Kong")
	if loc == nil {
		loc = time.UTC
	}

	// Parse start time (can be datetime string or Unix timestamp)
	var startTS int64
	var endTS int64

	if p.StartTime != nil {
		switch v := p.StartTime.(type) {
		case string:
			t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				// Try alternative formats
				t, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return "", fmt.Errorf("invalid start_time format: received '%s', expected 'YYYY-MM-DD HH:MM:SS'", v)
				}
				t = t.In(loc)
			}
			startTS = t.Unix()
		case float64:
			startTS = int64(v)
		}
	}

	if p.EndTime != nil {
		switch v := p.EndTime.(type) {
		case string:
			t, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				// Try alternative formats
				t, err = time.Parse(time.RFC3339, v)
				if err != nil {
					return "", fmt.Errorf("invalid end_time format: received '%s', expected 'YYYY-MM-DD HH:MM:SS'", v)
				}
				t = t.In(loc)
			}
			endTS = t.Unix()
		case float64:
			endTS = int64(v)
		}
	}

	// Adjust years if timestamp is in past year
	if startTS != 0 {
		startTS = adjustTimestampYear(startTS)
	}
	if endTS != 0 {
		endTS = adjustTimestampYear(endTS)
	}

	// Debug logging
	if startTS != 0 || endTS != 0 {
		log.Printf("[agent.debug] updateEvent start=%d (%s) end=%d (%s) user=%s id=%s",
			startTS, time.Unix(startTS, 0).In(loc).Format(time.RFC3339),
			endTS, time.Unix(endTS, 0).In(loc).Format(time.RFC3339),
			getUserID(ctx), p.ID,
		)
	}

	// If the caller provided any time updates, ensure final times fall within the allowed window.
	if startTS != 0 || endTS != 0 {
		// Build final start/end defaults if only one side was provided
		s := time.Unix(startTS, 0)
		e := time.Unix(endTS, 0)
		if startTS == 0 && endTS != 0 {
			// default start to one hour before end
			s = e.Add(-1 * time.Hour)
		}
		if endTS == 0 && startTS != 0 {
			// default end to one hour after start
			e = s.Add(time.Hour)
		}

		// Validate
		if err := validateEventWithinOneYear(s.Unix(), e.Unix()); err != nil {
			return "", fmt.Errorf("event time outside allowed window: %w", err)
		}

		// Use the (possibly filled) values
		startTS = s.Unix()
		endTS = e.Unix()
	}

	req := &pb.UpdateEventRequest{
		UserId:      getUserID(ctx),
		Id:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		StartTime:   startTS,
		EndTime:     endTS,
		Location:    p.Location,
		Attendees:   p.Attendees,
		Status:      p.Status,
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
	// Parse JSON input for id
	type payload struct {
		ID string `json:"id"`
	}
	var p payload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		return "", fmt.Errorf("invalid delete event payload: %w", err)
	}
	if p.ID == "" {
		return "", fmt.Errorf("missing event id")
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
	// Parse JSON input for range and duration
	type payload struct {
		StartTime int64 `json:"start_time"`
		EndTime   int64 `json:"end_time"`
		Duration  int64 `json:"duration"` // seconds
	}
	var p payload
	if input != "" {
		_ = json.Unmarshal([]byte(input), &p)
	}

	dur := int64(3600)
	if p.Duration > 0 {
		dur = p.Duration
	}

	// Debug: log parsed and adjusted timestamps for visibility (use raw/adj)
	rawStart := p.StartTime
	rawEnd := p.EndTime
	adjStart := adjustTimestampYear(p.StartTime)
	adjEnd := adjustTimestampYear(p.EndTime)
	if rawStart != 0 || rawEnd != 0 {
		loc, _ := time.LoadLocation("Asia/Hong_Kong")
		if loc == nil {
			loc = time.UTC
		}
		log.Printf("[agent.debug] getAvailableSlots parsed start=%d (%s) end=%d (%s) adjusted start=%d (%s) end=%d (%s) user=%s",
			rawStart, time.Unix(rawStart, 0).In(loc).Format(time.RFC3339),
			rawEnd, time.Unix(rawEnd, 0).In(loc).Format(time.RFC3339),
			adjStart, time.Unix(adjStart, 0).In(loc).Format(time.RFC3339),
			adjEnd, time.Unix(adjEnd, 0).In(loc).Format(time.RFC3339),
			getUserID(ctx),
		)
	}
	p.StartTime = adjStart
	p.EndTime = adjEnd

	// Clamp the requested range to [now, now+1 year]
	clampedStart, clampedEnd := clampRangeToOneYearWindow(p.StartTime, p.EndTime)
	// If clampedStart >= clampedEnd then no valid window; return empty result
	if clampedStart >= clampedEnd {
		return "Found 0 available slots", nil
	}

	req := &pb.GetAvailableSlotsRequest{
		UserId:    getUserID(ctx),
		StartTime: clampedStart,
		EndTime:   clampedEnd,
		Duration:  dur,
	}

	resp, err := t.client.GetAvailableSlots(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get available slots: %w", err)
	}

	return fmt.Sprintf("Found %d available slots", len(resp.Slots)), nil
}

// adjustTimestampYear nudges a parsed Unix timestamp forward by whole years
// ONLY if the timestamp's year is actually in the past (not just the datetime).
// This prevents "tomorrow 9am" from being pushed to next year if it's currently 10am.
func adjustTimestampYear(ts int64) int64 {
	if ts == 0 {
		return 0
	}
	// Load Hong Kong timezone for consistent time operations
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		loc = time.UTC
	}

	// Convert to time and compare with now in HK timezone
	t := time.Unix(ts, 0).In(loc)
	now := time.Now().In(loc)

	// Only adjust if the YEAR is in the past, not just if the datetime is in the past.
	// This prevents same-year past times (like "tomorrow 9am" when it's currently 10am)
	// from being pushed to next year.
	if t.Year() < now.Year() {
		// Add years to bring it to current year
		yearDiff := now.Year() - t.Year()
		t = t.AddDate(yearDiff, 0, 0)
		return t.Unix()
	}

	// If the datetime is in the past but the year is current or future,
	// don't adjust - let the validation layer handle it
	return ts
}

// clampRangeToOneYearWindow clamps a start/end Unix timestamp pair to the
// allowed window: [now, now+1 year]. If either timestamp is zero it will be
// replaced by sensible defaults (start -> now, end -> now+1h or start+1h).
// The function returns clampedStart, clampedEnd. If the clamped range is
// invalid (no overlap) both values may be equal which callers can treat as
// empty range.
func clampRangeToOneYearWindow(startTS, endTS int64) (int64, int64) {
	// Load Hong Kong timezone for consistent time operations
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	windowStart := now
	windowEnd := now.AddDate(1, 0, 0)

	// If neither timestamp provided, default to [now, now+1h]
	if startTS == 0 && endTS == 0 {
		s := windowStart
		e := s.Add(time.Hour)
		return s.Unix(), e.Unix()
	}

	var s time.Time
	var e time.Time

	if startTS != 0 {
		s = time.Unix(startTS, 0).In(loc)
	}
	if endTS != 0 {
		e = time.Unix(endTS, 0).In(loc)
	}

	// If start missing, but end provided -> default start to one hour before end
	if startTS == 0 {
		// endTS must be non-zero here because we handled both-zero above
		s = e.Add(-1 * time.Hour)
	}

	// If end missing, but start provided -> default end to one hour after start
	if endTS == 0 {
		// startTS must be non-zero here
		e = s.Add(time.Hour)
	}

	// Clamp to window
	if s.Before(windowStart) {
		s = windowStart
	}
	if e.After(windowEnd) {
		e = windowEnd
	}

	return s.Unix(), e.Unix()
}

// validateEventWithinOneYear ensures an event (start,end) falls within the
// allowed [now-5min, now+1year] window and that start < end. Returns an error if
// the event is invalid or outside the window. We allow a 5-minute grace period
// in the past to handle processing delays and clock skew.
func validateEventWithinOneYear(startTS, endTS int64) error {
	// Load Hong Kong timezone for consistent time operations
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	// Allow 5 minute grace period for times slightly in the past
	windowStart := now.Add(-5 * time.Minute).Unix()
	windowEnd := now.AddDate(1, 0, 0).Unix()

	if startTS == 0 || endTS == 0 {
		return fmt.Errorf("start and end times must be provided")
	}

	if startTS >= endTS {
		return fmt.Errorf("event start must be before end")
	}

	// Check if event is too far in the past
	if startTS < windowStart {
		startTime := time.Unix(startTS, 0).In(loc)
		return fmt.Errorf("event start time %s is too far in the past (current time: %s)",
			startTime.Format(time.RFC3339), now.Format(time.RFC3339))
	}

	// Check if event is too far in the future
	if endTS > windowEnd {
		endTime := time.Unix(endTS, 0).In(loc)
		oneYearFromNow := now.AddDate(1, 0, 0)
		return fmt.Errorf("event end time %s is beyond the allowed 1-year window (max: %s)",
			endTime.Format(time.RFC3339), oneYearFromNow.Format(time.RFC3339))
	}

	return nil
}

// TODO: migrate from langchaingo tools to the richer PlannerChatbotAgent
// multi-agent stack defined in this package. For now, the Agent.Chat method
// uses langchaingo Executor with calendar tools that already call the real
// gRPC calendar service (no mocked data).
