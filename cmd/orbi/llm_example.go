package main

import (
	"context"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-Orbi/pkg/llm"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi"
)

// Example: How to integrate the LLM client into your Orbi chatbot module

// LLMProvider wraps the Redis-based LLM client
type LLMProvider struct {
	client *llm.Client
}

// NewLLMProvider creates a new LLM provider connected to Redis
func NewLLMProvider(redisURL string) (*LLMProvider, error) {
	client, err := llm.NewClient(
		redisURL,
		"llm:queue:requests",           // Request queue name
		"mistral-7b-awq",               // Default model
		30*time.Second,                 // Default timeout
	)
	if err != nil {
		return nil, err
	}

	return &LLMProvider{client: client}, nil
}

// Inference submits a prompt and returns the model's response
func (p *LLMProvider) Inference(ctx context.Context, prompt string, opts ...InferenceOption) (string, error) {
	// Apply options
	req := llm.AskRequest{
		Prompt:      prompt,
		Temperature: floatPtr(0.2),
		MaxTokens:   int32Ptr(512),
		Timeout:     30 * time.Second,
	}

	for _, opt := range opts {
		opt(&req)
	}

	// Call vLLM via Redis worker
	resp, err := p.client.Ask(ctx, req)
	if err != nil {
		return "", fmt.Errorf("inference failed: %w", err)
	}

	return resp.Text, nil
}

// InferenceOption is a functional option for inference requests
type InferenceOption func(*llm.AskRequest)

// WithTemperature sets the sampling temperature
func WithTemperature(t float32) InferenceOption {
	return func(r *llm.AskRequest) {
		r.Temperature = &t
	}
}

// WithMaxTokens sets the maximum output tokens
func WithMaxTokens(n int32) InferenceOption {
	return func(r *llm.AskRequest) {
		r.MaxTokens = &n
	}
}

// WithTimeout sets the request timeout
func WithTimeout(d time.Duration) InferenceOption {
	return func(r *llm.AskRequest) {
		r.Timeout = d
	}
}

// WithModel sets the model to use
func WithModel(model string) InferenceOption {
	return func(r *llm.AskRequest) {
		r.Model = model
	}
}

// Close closes the underlying Redis connection
func (p *LLMProvider) Close() error {
	return p.client.Close()
}

// Helper functions
func floatPtr(f float32) *float32 {
	return &f
}

func int32Ptr(i int32) *int32 {
	return &i
}

// Example usage in main
func ExampleUsage() {
	// Load dotenv file if present so example honors local .env configuration.
	_ = orbi.LoadDotEnv(".env")
	ctx := context.Background()

	// Initialize provider
	provider, err := NewLLMProvider("redis://localhost:6379/0")
	if err != nil {
		panic(err)
	}
	defer provider.Close()

	// Basic inference
	result, err := provider.Inference(ctx, "What is the capital of France?")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Result: %s\n", result)

	// With options
	result, err = provider.Inference(
		ctx,
		"Write a short poem about the moon",
		WithTemperature(0.7),
		WithMaxTokens(256),
		WithTimeout(60*time.Second),
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Poem:\n%s\n", result)
}

// Integration with LangChain-like chain pattern
type LLMChain struct {
	provider *LLMProvider
	prompt   string
}

// NewLLMChain creates a new chain with a template prompt
func NewLLMChain(provider *LLMProvider, promptTemplate string) *LLMChain {
	return &LLMChain{
		provider: provider,
		prompt:   promptTemplate,
	}
}

// Run executes the chain with the given input
func (c *LLMChain) Run(ctx context.Context, input map[string]string) (string, error) {
	// Format prompt with input variables
	prompt := c.prompt
	for _, value := range input {
		prompt = fmt.Sprintf(prompt, value)
	}

	// Call LLM
	return c.provider.Inference(ctx, prompt)
}

// Example LLM chain usage
func ExampleChainUsage() {
	ctx := context.Background()

	provider, _ := NewLLMProvider("redis://localhost:6379/0")
	defer provider.Close()

	// Create a chain with a prompt template
	chain := NewLLMChain(
		provider,
		"Q: %s\nA:",
	)

	// Execute the chain
	result, err := chain.Run(ctx, map[string]string{
		"question": "What is 2+2?",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Chain result: %s\n", result)
}
