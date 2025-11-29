package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/config"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/llm"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/memory"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/tools"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/types"
	"google.golang.org/grpc/metadata"
)

// CalendarAgent represents the Orbi chatbot agent
type CalendarAgent struct {
	cfg      config.Config
	llm      llms.Model
	executor *agents.Executor
	memory   memory.AgentMemory

	mu          sync.Mutex // protects the following
	initialized bool
}

// NewCalendarAgent creates a new Orbi agent but does NOT initialize heavy resources
func NewCalendarAgent(cfg config.Config) (*CalendarAgent, error) {
	agent := &CalendarAgent{
		cfg: cfg,
	}
	return agent, nil
}

// ensureInitialized performs one-time initialization of heavy resources.
func (a *CalendarAgent) ensureInitialized(ctx context.Context) error {
	if a.initialized {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.initialized {
		return nil
	}

	// Initialize Memory (Redis or In-Memory fallback)
	var mem memory.AgentMemory
	if a.cfg.RedisAddr != "" {
		mem = memory.NewRedisAgentMemory(a.cfg.RedisAddr, a.cfg.RedisPassword, a.cfg.RedisDB)
	} else {
		mem = memory.NewInMemoryAgentMemory()
	}
	a.memory = mem

	// Initialize LLM
	llmModel, err := llm.New(a.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize LLM: %w", err)
	}
	a.llm = llmModel

	// Create tools
	calendarTools, err := tools.NewCalendarTools(a.cfg.CalendarServiceAddr, a.cfg.Timezone)
	if err != nil {
		return fmt.Errorf("failed to create calendar tools: %w", err)
	}

	// Create agent executor
	executor := agents.NewExecutor(
		agents.NewConversationalAgent(a.llm, calendarTools),
		agents.WithMaxIterations(5),
	)
	a.executor = executor

	a.initialized = true
	return nil
}

// Chat processes a user message and returns a response.
func (a *CalendarAgent) Chat(ctx context.Context, message string) (string, error) {
	if err := a.ensureInitialized(ctx); err != nil {
		return "", err
	}

	loc := time.UTC
	if a.cfg.Timezone != "" {
		if parsedLoc, err := time.LoadLocation(a.cfg.Timezone); err == nil {
			loc = parsedLoc
		} else {
			log.Printf("failed to load timezone %q, defaulting to UTC: %v", a.cfg.Timezone, err)
		}
	}
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("2006-01-02 15:04:05")
	timezoneName := loc.String()

	userID := getUserID(ctx)
	if userID == "" {
		userID = "default_user"
	}

	var historyStr string
	if msgs, err := a.memory.GetMessages(ctx, userID, 10); err == nil && len(msgs) > 0 {
		historyStr = "Conversation History:\n"
		for _, m := range msgs {
			historyStr += fmt.Sprintf("- %s: %s\n", m.Role, m.Content)
		}
		historyStr += "\n"
	}

	augmented := fmt.Sprintf(`Current Date and Time: %s (Timezone: %s)
%s

When you are asked to update an event, you should first use the "search_events" tool to find the event's ID. Then, you can use the "update_event" tool to update the event with the correct ID.

User: %s`, currentTimeStr, timezoneName, historyStr, message)

	result, err := chains.Run(ctx, a.executor, augmented)
	if err != nil {
		return "", fmt.Errorf("failed to process message: %w", err)
	}

	if err := a.memory.SaveMessage(ctx, userID, types.Message{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}); err != nil {
		log.Printf("failed to save user message: %v", err)
	}
	if err := a.memory.SaveMessage(ctx, userID, types.Message{
		Role:      "assistant",
		Content:   result,
		Timestamp: time.Now(),
	}); err != nil {
		log.Printf("failed to save assistant message: %v", err)
	}

	return result, nil
}

// Close closes the agent and its resources if they were initialized
func (a *CalendarAgent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.memory != nil {
		if redisMem, ok := a.memory.(*memory.RedisAgentMemory); ok {
			_ = redisMem.Close()
		}
	}

	a.llm = nil
	a.executor = nil
	a.memory = nil
	a.initialized = false

	return nil
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
