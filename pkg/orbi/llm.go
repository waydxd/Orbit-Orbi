package orbi

import "context"

// LLMInterface defines the contract for LLM providers
type LLMInterface interface {
	// Generate creates a text response from a prompt
	Generate(ctx context.Context, prompt string) (string, error)

	// GenerateWithOptions creates a response with specified options
	GenerateWithOptions(ctx context.Context, prompt string, opts LLMOptions) (string, error)

	// Close closes the LLM connection if needed
	Close() error
}

// LLMOptions contains options for LLM generation
type LLMOptions struct {
	Temperature      float32
	MaxTokens        int32
	TopP             float32
	FrequencyPenalty float32
	PresencePenalty  float32
}

// DefaultLLMOptions returns default options for LLM generation
func DefaultLLMOptions() LLMOptions {
	return LLMOptions{
		Temperature:      0.7,
		MaxTokens:        512,
		TopP:             0.9,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
	}
}

// SimpleLLMAdapter is a basic wrapper for OpenAI LLM
// This can be replaced with actual LLM implementations
type SimpleLLMAdapter struct {
	generateFunc func(ctx context.Context, prompt string) (string, error)
}

// NewSimpleLLMAdapter creates a new LLM adapter
func NewSimpleLLMAdapter(generateFunc func(ctx context.Context, prompt string) (string, error)) *SimpleLLMAdapter {
	return &SimpleLLMAdapter{
		generateFunc: generateFunc,
	}
}

// Generate implements the LLMInterface
func (a *SimpleLLMAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.generateFunc(ctx, prompt)
}

// GenerateWithOptions implements the LLMInterface
func (a *SimpleLLMAdapter) GenerateWithOptions(ctx context.Context, prompt string, opts LLMOptions) (string, error) {
	// For now, ignore options and use the default generation
	return a.generateFunc(ctx, prompt)
}

// Close implements the LLMInterface
func (a *SimpleLLMAdapter) Close() error {
	return nil
}

// MockLLM is a mock implementation for testing
type MockLLM struct {
	responses map[string]string
}

// NewMockLLM creates a new mock LLM
func NewMockLLM() *MockLLM {
	return &MockLLM{
		responses: make(map[string]string),
	}
}

// AddResponse adds a mock response for a prompt
func (m *MockLLM) AddResponse(prompt string, response string) {
	m.responses[prompt] = response
}

// Generate implements the LLMInterface
func (m *MockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	if resp, ok := m.responses[prompt]; ok {
		return resp, nil
	}
	return "Mock response", nil
}

// GenerateWithOptions implements the LLMInterface
func (m *MockLLM) GenerateWithOptions(ctx context.Context, prompt string, opts LLMOptions) (string, error) {
	return m.Generate(ctx, prompt)
}

// Close implements the LLMInterface
func (m *MockLLM) Close() error {
	return nil
}
