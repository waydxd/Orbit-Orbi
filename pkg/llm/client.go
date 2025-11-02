package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// JobPayload represents the LLM inference job payload
type JobPayload struct {
	Prompt string `json:"prompt"`
}

// Job represents a complete LLM inference job
type Job struct {
	JobID     string                 `json:"job_id"`
	Timestamp string                 `json:"timestamp"`
	Priority  int                    `json:"priority"`
	Model     string                 `json:"model"`
	Params    map[string]interface{} `json:"params"`
	Payload   JobPayload             `json:"payload"`
	Trace     map[string]interface{} `json:"trace"`
}

// ResultOK represents a successful inference result
type ResultOK struct {
	JobID      string                 `json:"job_id"`
	Status     string                 `json:"status"`
	Output     map[string]string      `json:"output"`
	Usage      map[string]interface{} `json:"usage"`
	WorkerMeta map[string]interface{} `json:"worker_meta"`
	LatencyMS  int64                  `json:"latency_ms"`
}

// ErrorInfo represents an error response
type ErrorInfo struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details"`
}

// ResultError represents a failed inference result
type ResultError struct {
	JobID      string                 `json:"job_id"`
	Status     string                 `json:"status"`
	Error      ErrorInfo              `json:"error"`
	WorkerMeta map[string]interface{} `json:"worker_meta"`
}

// Client manages communication with the vLLM inference pipeline via Redis
type Client struct {
	redis          *redis.Client
	requestQueue   string
	defaultModel   string
	defaultTimeout time.Duration
}

// NewClient creates a new LLM client connected to Redis
func NewClient(redisURL, requestQueue, defaultModel string, defaultTimeout time.Duration) (*Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	rdb := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{
		redis:          rdb,
		requestQueue:   requestQueue,
		defaultModel:   defaultModel,
		defaultTimeout: defaultTimeout,
	}, nil
}

// AskRequest represents the parameters for an inference request
type AskRequest struct {
	Prompt      string
	Model       string
	Temperature *float32
	MaxTokens   *int32
	Timeout     time.Duration
}

// AskResponse represents the response from an inference request
type AskResponse struct {
	JobID     string
	Text      string
	Usage     map[string]interface{}
	LatencyMS int64
}

// Ask submits a prompt for inference and waits for the result
func (c *Client) Ask(ctx context.Context, req AskRequest) (*AskResponse, error) {
	// Set defaults
	if req.Model == "" {
		req.Model = c.defaultModel
	}
	if req.Timeout == 0 {
		req.Timeout = c.defaultTimeout
	}

	// Create job
	job := Job{
		JobID:     uuid.New().String(),
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
		Priority:  0,
		Model:     req.Model,
		Params: map[string]interface{}{
			"temperature": 0.2,
			"max_tokens":  512,
		},
		Payload: JobPayload{Prompt: req.Prompt},
		Trace:   map[string]interface{}{},
	}

	// Override params if provided
	if req.Temperature != nil {
		job.Params["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		job.Params["max_tokens"] = *req.MaxTokens
	}

	// Serialize and enqueue
	jobJSON, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.redis.LPush(jobCtx, c.requestQueue, string(jobJSON)).Err(); err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Wait for result
	resultKey := fmt.Sprintf("llm:result:%s", job.JobID)
	deadline := time.Now().Add(req.Timeout)

	for {
		now := time.Now()
		if now.After(deadline) {
			return nil, fmt.Errorf("inference timeout (>%v)", req.Timeout)
		}

		remaining := time.Until(deadline)
		blockTimeout := remaining
		if blockTimeout > 1*time.Second {
			blockTimeout = 1 * time.Second
		}

		resultCtx, cancel := context.WithTimeout(ctx, blockTimeout+1*time.Second)
		result, err := c.redis.BRPop(resultCtx, blockTimeout, resultKey).Result()
		cancel()

		if err == redis.Nil {
			// Timeout, no result yet
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve result: %w", err)
		}

		// Parse result
		if len(result) < 2 {
			return nil, fmt.Errorf("unexpected result format from Redis")
		}

		payload := result[1]

		// Try to unmarshal as success result first
		var resultOK ResultOK
		if err := json.Unmarshal([]byte(payload), &resultOK); err == nil && resultOK.Status == "ok" {
			return &AskResponse{
				JobID:     resultOK.JobID,
				Text:      resultOK.Output["text"],
				Usage:     resultOK.Usage,
				LatencyMS: resultOK.LatencyMS,
			}, nil
		}

		// Try to unmarshal as error result
		var resultErr ResultError
		if err := json.Unmarshal([]byte(payload), &resultErr); err == nil && resultErr.Status == "error" {
			return nil, fmt.Errorf("%s: %s", resultErr.Error.Code, resultErr.Error.Message)
		}

		// Unknown result format
		return nil, fmt.Errorf("unknown result format: %s", payload)
	}
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.redis.Close()
}
