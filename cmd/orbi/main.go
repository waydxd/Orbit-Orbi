package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/waydxd/Orbit-Orbi/pkg/orbi"
)

func main() {
	// Get configuration from environment variables
	calendarAddr := getEnv("CALENDAR_SERVICE_ADDR", "localhost:50051")
	openAIKey := getEnv("OPENAI_API_KEY", "")
	model := getEnv("OPENAI_MODEL", "gpt-3.5-turbo")

	if openAIKey == "" {
		log.Println("Warning: OPENAI_API_KEY not set. Agent will not function without it.")
		log.Println("Please set the environment variable before running in production.")
	}

	// Create agent configuration
	cfg := orbi.Config{
		CalendarServiceAddr: calendarAddr,
		OpenAIAPIKey:        openAIKey,
		Model:               model,
	}

	// Initialize the Orbi agent
	log.Println("Initializing Orbi agent...")
	agent, err := orbi.NewAgent(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	log.Println("Orbi agent initialized successfully!")
	log.Println("Type 'exit' or 'quit' to exit the chat.")
	log.Println()

	// Start interactive chat loop
	reader := bufio.NewReader(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Check for exit commands
		if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
			log.Println("Goodbye!")
			break
		}

		// Process the message with the agent
		response, err := agent.Chat(ctx, input)
		if err != nil {
			log.Printf("Error processing message: %v", err)
			continue
		}

		fmt.Printf("Orbi: %s\n\n", response)
	}
}

// getEnv retrieves an environment variable with a default fallback
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
