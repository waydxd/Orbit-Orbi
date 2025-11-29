package examples

import (
	"context"
	"fmt"

	"github.com/waydxd/Orbit-Orbi/pkg/orbi/agent"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/config"
)

// ExampleUsage demonstrates how to use the new simplified CalendarAgent
func ExampleUsage() {
	ctx := context.Background()

	// 1. Setup configuration
	cfg := config.Config{
		CalendarServiceAddr: "localhost:50051",
		OpenAIAPIKey:        "your-api-key",
		Model:               "gpt-4",
		Timezone:            "Asia/Hong_Kong",
	}

	// 2. Create the CalendarAgent
	calendarAgent, err := agent.NewCalendarAgent(cfg)
	if err != nil {
		fmt.Printf("Error creating agent: %v\n", err)
		return
	}
	defer calendarAgent.Close()

	// 3. Process user input
	response, err := calendarAgent.Chat(ctx, "What is my next meeting?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Response: %s\n", response)
}

// ExampleMultiTurnConversation demonstrates a multi-turn conversation
func ExampleMultiTurnConversation() {
	ctx := context.Background()

	// Setup configuration
	cfg := config.Config{
		CalendarServiceAddr: "localhost:50051",
		OpenAIAPIKey:        "your-api-key",
		Model:               "gpt-4",
		Timezone:            "Asia/Hong_Kong",
	}

	// Create the CalendarAgent
	calendarAgent, err := agent.NewCalendarAgent(cfg)
	if err != nil {
		fmt.Printf("Error creating agent: %v\n", err)
		return
	}
	defer calendarAgent.Close()

	// Turn 1: Query
	fmt.Println("=== Turn 1: Query ===")
	response1, err := calendarAgent.Chat(ctx, "What's on my calendar tomorrow?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("User: What's on my calendar tomorrow?\n")
	fmt.Printf("Orbi: %s\n\n", response1)

	// Turn 2: Update
	fmt.Println("=== Turn 2: Update ===")
	response2, err := calendarAgent.Chat(ctx, "Move my 10am meeting to 11am")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("User: Move my 10am meeting to 11am\n")
	fmt.Printf("Orbi: %s\n\n", response2)

	// Turn 3: Follow-up
	fmt.Println("=== Turn 3: Follow-up ===")
	response3, err := calendarAgent.Chat(ctx, "What does my calendar look like now?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("User: What does my calendar look like now?\n")
	fmt.Printf("Orbi: %s\n\n", response3)
}
