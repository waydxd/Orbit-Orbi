package orbi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RetrievalAgentImpl is the concrete implementation of the Retrieval Agent
type RetrievalAgentImpl struct {
	name      string
	tool      RetrievalTool
	llm       LLMInterface
	validator ConfirmationValidator
}

// NewRetrievalAgent creates a new Retrieval Agent instance
func NewRetrievalAgent(tool RetrievalTool, llm LLMInterface, validator ConfirmationValidator) *RetrievalAgentImpl {
	return &RetrievalAgentImpl{
		name:      "retrieval-agent",
		tool:      tool,
		llm:       llm,
		validator: validator,
	}
}

// Name returns the agent's identifier
func (a *RetrievalAgentImpl) Name() string {
	return a.name
}

// ShouldHandle returns true if this agent should handle the given intent
func (a *RetrievalAgentImpl) ShouldHandle(intent *Intent) bool {
	return intent.Type == "query" || intent.Type == "read"
}

// Handle processes a retrieval request
func (a *RetrievalAgentImpl) Handle(ctx context.Context, intent *Intent, context *ConversationContext) (string, error) {
	action := intent.Action
	params := intent.Parameters

	switch action {
	case "get_events":
		return a.handleGetEvents(ctx, params, context)
	case "get_available_slots":
		return a.handleGetAvailableSlots(ctx, params, context)
	case "get_event_details":
		return a.handleGetEventDetails(ctx, params, context)
	case "query_history":
		return a.handleQueryHistory(ctx, params, context)
	case "search_events":
		return a.handleSearchEvents(ctx, params, context)
	default:
		return "", fmt.Errorf("unknown retrieval action: %s", action)
	}
}

// handleGetEvents retrieves events in a time range
func (a *RetrievalAgentImpl) handleGetEvents(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	// Extract parameters
	startTime, ok := params["start_time"].(time.Time)
	if !ok {
		// Try to parse from string or int64
		if s, ok := params["start_time"].(string); ok {
			var err error
			startTime, err = time.Parse(time.RFC3339, s)
			if err != nil {
				return "", fmt.Errorf("invalid start_time: %w", err)
			}
		} else if ts, ok := params["start_time"].(int64); ok {
			startTime = time.Unix(ts, 0)
		} else {
			startTime = time.Now()
		}
	}

	endTime, ok := params["end_time"].(time.Time)
	if !ok {
		if s, ok := params["end_time"].(string); ok {
			var err error
			endTime, err = time.Parse(time.RFC3339, s)
			if err != nil {
				return "", fmt.Errorf("invalid end_time: %w", err)
			}
		} else if ts, ok := params["end_time"].(int64); ok {
			endTime = time.Unix(ts, 0)
		} else {
			endTime = time.Now().Add(24 * time.Hour)
		}
	}

	// Extract filters
	filters := make(map[string]interface{})
	if f, ok := params["filters"]; ok {
		filters = f.(map[string]interface{})
	}

	// Query events
	events, err := a.tool.GetEvents(ctx, startTime, endTime, filters)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve events: %w", err)
	}

	// Format response
	if len(events) == 0 {
		return "No events found in the specified time range.", nil
	}

	// Use LLM to generate a natural language summary
	summary, err := a.generateEventsSummary(ctx, events, startTime, endTime)
	if err != nil {
		// Fallback to JSON if LLM fails
		data, _ := json.MarshalIndent(events, "", "  ")
		return string(data), nil
	}

	return summary, nil
}

// handleGetAvailableSlots finds free time slots
func (a *RetrievalAgentImpl) handleGetAvailableSlots(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	startTime, ok := params["start_time"].(time.Time)
	if !ok {
		if s, ok := params["start_time"].(string); ok {
			var err error
			startTime, err = time.Parse(time.RFC3339, s)
			if err != nil {
				return "", fmt.Errorf("invalid start_time: %w", err)
			}
		} else {
			startTime = time.Now()
		}
	}

	endTime, ok := params["end_time"].(time.Time)
	if !ok {
		if s, ok := params["end_time"].(string); ok {
			var err error
			endTime, err = time.Parse(time.RFC3339, s)
			if err != nil {
				return "", fmt.Errorf("invalid end_time: %w", err)
			}
		} else {
			endTime = time.Now().Add(24 * time.Hour)
		}
	}

	duration := 1 * time.Hour // default
	if d, ok := params["duration"].(time.Duration); ok {
		duration = d
	} else if d, ok := params["duration"].(int64); ok {
		duration = time.Duration(d) * time.Minute
	}

	// Get available slots
	slots, err := a.tool.GetAvailableSlots(ctx, startTime, endTime, duration)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve available slots: %w", err)
	}

	if len(slots) == 0 {
		return fmt.Sprintf("No available slots found for a %v meeting between %v and %v.",
			duration, startTime.Format(time.RFC1123), endTime.Format(time.RFC1123)), nil
	}

	// Use LLM to generate a natural language summary
	summary, err := a.generateSlotsSummary(ctx, slots, startTime, endTime, duration)
	if err != nil {
		data, _ := json.MarshalIndent(slots, "", "  ")
		return string(data), nil
	}

	return summary, nil
}

// handleGetEventDetails retrieves full details of an event
func (a *RetrievalAgentImpl) handleGetEventDetails(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	eventID, ok := params["event_id"].(string)
	if !ok {
		return "", fmt.Errorf("missing event_id parameter")
	}

	event, err := a.tool.GetEventDetails(ctx, eventID)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve event details: %w", err)
	}

	// Format the response
	summary, err := a.generateEventDetail(ctx, event)
	if err != nil {
		data, _ := json.MarshalIndent(event, "", "  ")
		return string(data), nil
	}

	return summary, nil
}

// handleQueryHistory retrieves past interactions
func (a *RetrievalAgentImpl) handleQueryHistory(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	limit := 10
	if l, ok := params["limit"].(int); ok {
		limit = l
	}

	messages, err := a.tool.QueryHistory(ctx, context.SessionID, limit)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve history: %w", err)
	}

	if len(messages) == 0 {
		return "No conversation history found.", nil
	}

	// Format conversation history
	summary := fmt.Sprintf("Retrieved %d recent messages from conversation history:\n\n", len(messages))
	for i, msg := range messages {
		role := "You"
		if msg.Role == "assistant" {
			role = "Orbi"
		}
		summary += fmt.Sprintf("%d. [%s] %s: %s\n", i+1, msg.Timestamp.Format(time.RFC1123), role, msg.Content)
	}

	return summary, nil
}

// handleSearchEvents searches events with multiple criteria
func (a *RetrievalAgentImpl) handleSearchEvents(
	ctx context.Context,
	params map[string]interface{},
	context *ConversationContext,
) (string, error) {
	// Build search parameters
	query := ""
	if q, ok := params["query"].(string); ok {
		query = q
	}

	// Extract time range if provided
	startTime := time.Now()
	if s, ok := params["start_time"]; ok {
		if st, ok := s.(time.Time); ok {
			startTime = st
		}
	}

	endTime := time.Now().Add(30 * 24 * time.Hour) // default: next 30 days
	if e, ok := params["end_time"]; ok {
		if et, ok := e.(time.Time); ok {
			endTime = et
		}
	}

	filters := make(map[string]interface{})
	filters["query"] = query
	if at, ok := params["attendee"]; ok {
		filters["attendee"] = at
	}
	if loc, ok := params["location"]; ok {
		filters["location"] = loc
	}

	// Query events
	events, err := a.tool.GetEvents(ctx, startTime, endTime, filters)
	if err != nil {
		return "", fmt.Errorf("failed to search events: %w", err)
	}

	if len(events) == 0 {
		return fmt.Sprintf("No events found matching '%s'.", query), nil
	}

	summary, err := a.generateEventsSummary(ctx, events, startTime, endTime)
	if err != nil {
		data, _ := json.MarshalIndent(events, "", "  ")
		return string(data), nil
	}

	return summary, nil
}

// generateEventsSummary uses the LLM to create a natural language summary of events
func (a *RetrievalAgentImpl) generateEventsSummary(
	ctx context.Context,
	events []map[string]interface{},
	startTime, endTime time.Time,
) (string, error) {
	if a.llm == nil {
		// Fallback: return JSON
		data, _ := json.MarshalIndent(events, "", "  ")
		return string(data), nil
	}

	// Prepare data for LLM
	eventData, _ := json.MarshalIndent(events, "", "  ")
	prompt := fmt.Sprintf(`You are a helpful assistant for a smart calendar system. 
A user asked about their events.

Events found (%d total):
%s

Time range: %s to %s

Generate a brief, natural language summary of these events. 
Be concise but informative. If there are many events, highlight the important ones.
Respond in the user's language if it was in Chinese.`,
		len(events), string(eventData), startTime.Format(time.RFC1123), endTime.Format(time.RFC1123))

	response, err := a.llm.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// generateSlotsSummary uses the LLM to create a natural language summary of available slots
func (a *RetrievalAgentImpl) generateSlotsSummary(
	ctx context.Context,
	slots []map[string]interface{},
	startTime, endTime time.Time,
	duration time.Duration,
) (string, error) {
	if a.llm == nil {
		data, _ := json.MarshalIndent(slots, "", "  ")
		return string(data), nil
	}

	slotData, _ := json.MarshalIndent(slots, "", "  ")
	prompt := fmt.Sprintf(`You are a helpful assistant for a smart calendar system.
A user asked for available time slots.

Available slots found (%d total) for %v meetings:
%s

Time range: %s to %s

Generate a brief, natural language summary of the best available slots.
Highlight the earliest and most convenient times.`,
		len(slots), duration, string(slotData), startTime.Format(time.RFC1123), endTime.Format(time.RFC1123))

	response, err := a.llm.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}

	return response, nil
}

// generateEventDetail uses the LLM to create a natural language summary of an event
func (a *RetrievalAgentImpl) generateEventDetail(
	ctx context.Context,
	event map[string]interface{},
) (string, error) {
	if a.llm == nil {
		data, _ := json.MarshalIndent(event, "", "  ")
		return string(data), nil
	}

	eventData, _ := json.MarshalIndent(event, "", "  ")
	prompt := fmt.Sprintf(`You are a helpful assistant for a smart calendar system.
A user asked for details about a specific event.

Event details:
%s

Generate a brief, natural language summary of this event including:
- Event title and description
- Date and time
- Location (if any)
- Attendees (if any)
- Any other relevant information`,
		string(eventData))

	response, err := a.llm.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}

	return response, nil
}
