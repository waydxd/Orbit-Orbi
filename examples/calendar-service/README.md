# Example Calendar Service

This is a simple example implementation of a calendar service that works with Orbi.

## Overview

This example service implements the gRPC CalendarService defined in `proto/calendar.proto`. It provides:

- In-memory storage of calendar events
- All CRUD operations for events
- Available time slot calculation

## Running the Example

1. Build and run the calendar service:
```bash
cd examples/calendar-service
go run main.go
```

The service will start on port 50051.

2. In another terminal, run Orbi:
```bash
cd ../..
export CALENDAR_SERVICE_ADDR=localhost:50051
export OPENAI_API_KEY=your-api-key
./bin/orbi
```

## Implementation Notes

This is a **simple example** meant for demonstration purposes. A production calendar service would need:

- Persistent storage (database)
- User authentication and authorization
- Proper time zone handling
- Conflict detection for overlapping events
- Recurring event support
- Calendar sharing and permissions
- Integration with external calendar systems (Google Calendar, Outlook, etc.)

## Using with Orbi

Once both services are running, you can interact with the calendar through natural language:

```
You: Create a meeting tomorrow at 2pm
Orbi: I've created a meeting for tomorrow at 2:00 PM.

You: Show me my events this week
Orbi: You have 3 events scheduled this week...

You: Find me a free hour next Tuesday
Orbi: I found these available slots next Tuesday...
```
