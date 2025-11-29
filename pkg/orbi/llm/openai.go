package llm

import (
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/config"
)

// New returns a new OpenAI LLM.
func New(cfg config.Config) (llms.Model, error) {
	llm, err := openai.New(
		openai.WithBaseURL(cfg.BaseURL),
		openai.WithToken(cfg.OpenAIAPIKey),
		openai.WithModel(cfg.Model),
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithEmbeddingModel(cfg.Model),
	)
	if err != nil {
		return nil, err
	}
	return llm, nil
}
