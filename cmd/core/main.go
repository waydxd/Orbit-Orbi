package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/waydxd/Orbit-Orbi/pkg/grpcclient"
	"github.com/google/uuid"
)

func main() {
	// Get agent address from environment or use default
	agentAddr := os.Getenv("AGENT_SERVICE_ADDR")
	if agentAddr == "" {
		agentAddr = "localhost:50052"
	}

	log.Printf("Connecting to Agent at %s...", agentAddr)

	// Connect to the Agent gRPC service
	client, err := grpcclient.NewAgentClient(agentAddr)
	if err != nil {
		log.Fatalf("Failed to connect to agent: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Check agent health
	state, err := client.GetAgentState(ctx, "")
	if err != nil {
		log.Fatalf("Failed to check agent state: %v", err)
	}

	if !state.Ready {
		log.Fatalf("Agent is not ready: %s", state.Message)
	}

	log.Println("✓ Connected to Agent successfully!")
	log.Printf("Agent Status: %s - %s", state.Status, state.Message)
	log.Println("\nCore CLI - Chat with Orbi Agent")
	log.Println("Type 'exit' or 'quit' to exit.")
	log.Println()

	// Generate session ID for tracking
	sessionID := uuid.New().String()

	// Interactive chat loop
	reader := bufio.NewReader(os.Stdin)

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

		// Send message to agent via gRPC
		response, err := client.ProcessMessage(ctx, input, sessionID)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Printf("Orbi: %s\n\n", response)
	}
}
