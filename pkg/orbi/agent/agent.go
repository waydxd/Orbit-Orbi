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
func (a *CalendarAgent) ensureInitialized() error {
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
	if err := a.ensureInitialized(); err != nil {
		log.Printf("[Chat] Initialization failed: %v", err)
		return "", err
	}

	loc := time.FixedZone("HKT", 8*60*60)
	if a.cfg.Timezone != "" {
		if parsedLoc, err := time.LoadLocation(a.cfg.Timezone); err == nil {
			loc = parsedLoc
		} else {
			log.Printf("[Chat] Failed to load timezone %q, defaulting to HKT: %v", a.cfg.Timezone, err)
		}
	}
	currentTime := time.Now().In(loc)
	currentTimeStr := currentTime.Format("2006-01-02 15:04:05")
	timezoneName := loc.String()

	userID := getUserID(ctx)
	if userID == "" {
		userID = "default_user"
	}

	log.Printf("[Chat] Processing message for user_id=%q message_len=%d", userID, len(message))

	var historyStr string
	if msgs, err := a.memory.GetMessages(ctx, userID, 10); err == nil && len(msgs) > 0 {
		historyStr = "Conversation History:\n"
		for _, m := range msgs {
			historyStr += fmt.Sprintf("- %s: %s\n", m.Role, m.Content)
		}
		historyStr += "\n"
	} else if err != nil {
		log.Printf("[Chat] WARNING: Failed to load conversation history for user_id=%q: %v", userID, err)
	}

	augmented := fmt.Sprintf(`Current Date and Time: %s (Timezone: %s)
%s

If user intent is to create/add/new event/task, use "create_event" even if an event with same title already exists.
Use "update_event" when the user explicitly asks to modify/reschedule/change an existing event.
Do not update an event just because titles are similar.
When you are asked to update an event, you should first use the "search_events" tool to find the event's ID. Then, you can use the "update_event" tool to update the event with the correct ID.
User: %s`, currentTimeStr, timezoneName, historyStr, message)

	log.Printf("[Chat] Invoking LangChain agent executor for user_id=%q", userID)
	result, err := chains.Run(ctx, a.executor, augmented)
	if err != nil {
		log.Printf("[Chat] ERROR: chains.Run failed for user_id=%q — %v", userID, err)
		log.Printf("[Chat] ERROR: This is likely caused by a calendar backend (gRPC) error propagating through the tool chain. Check calendar tool logs above for details.")
		return "", fmt.Errorf("failed to process message: %w", err)
	}

	log.Printf("[Chat] Agent returned response for user_id=%q response_len=%d", userID, len(result))

	if err := a.memory.SaveMessage(ctx, userID, types.Message{
		Role:      "user",
		Content:   message,
		Timestamp: time.Now(),
	}); err != nil {
		log.Printf("[Chat] WARNING: Failed to save user message for user_id=%q: %v", userID, err)
	}
	if err := a.memory.SaveMessage(ctx, userID, types.Message{
		Role:      "assistant",
		Content:   result,
		Timestamp: time.Now(),
	}); err != nil {
		log.Printf("[Chat] WARNING: Failed to save assistant message for user_id=%q: %v", userID, err)
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
