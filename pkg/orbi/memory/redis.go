package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/waydxd/Orbit-Orbi/pkg/orbi/types"
)

// RedisAgentMemory implements AgentMemory using Redis
type RedisAgentMemory struct {
	client *redis.Client
	ttl    time.Duration
}

const (
	msgKeyPrefix    = "orbi:session:msgs:"
	intentKeyPrefix = "orbi:session:intents:"
	defaultTTL      = 24 * time.Hour
)

// NewRedisAgentMemory creates a new Redis-backed memory store
func NewRedisAgentMemory(addr, password string, db int) *RedisAgentMemory {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisAgentMemory{
		client: rdb,
		ttl:    defaultTTL,
	}
}

// SaveMessage adds a message to history
func (m *RedisAgentMemory) SaveMessage(ctx context.Context, sessionID string, msg types.Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	key := msgKeyPrefix + sessionID
	pipe := m.client.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, m.ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save message to redis: %w", err)
	}
	return nil
}

// GetMessages retrieves conversation history
func (m *RedisAgentMemory) GetMessages(ctx context.Context, sessionID string, limit int) ([]types.Message, error) {
	key := msgKeyPrefix + sessionID

	var start int64 = 0
	if limit > 0 {
		// To get the last N messages, we start from -N
		start = int64(-limit)
	}

	vals, err := m.client.LRange(ctx, key, start, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages from redis: %w", err)
	}

	msgs := make([]types.Message, 0, len(vals))
	for _, v := range vals {
		var msg types.Message
		if err := json.Unmarshal([]byte(v), &msg); err != nil {
			continue // Skip malformed messages
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// ClearSession removes a session
func (m *RedisAgentMemory) ClearSession(ctx context.Context, sessionID string) error {
	msgKey := msgKeyPrefix + sessionID
	intentKey := intentKeyPrefix + sessionID

	pipe := m.client.Pipeline()
	pipe.Del(ctx, msgKey)
	pipe.Del(ctx, intentKey)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to clear session in redis: %w", err)
	}
	return nil
}

// SaveIntent caches an identified intent
func (m *RedisAgentMemory) SaveIntent(ctx context.Context, sessionID string, intent *types.Intent) error {
	data, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("failed to marshal intent: %w", err)
	}

	key := intentKeyPrefix + sessionID
	pipe := m.client.Pipeline()
	// We save as "last" to match InMemory implementation behavior
	pipe.HSet(ctx, key, "last", data)
	pipe.Expire(ctx, key, m.ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to save intent to redis: %w", err)
	}
	return nil
}

// GetIntent retrieves a cached intent
func (m *RedisAgentMemory) GetIntent(ctx context.Context, sessionID string, key string) (*types.Intent, error) {
	redisKey := intentKeyPrefix + sessionID

	val, err := m.client.HGet(ctx, redisKey, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("intent not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get intent from redis: %w", err)
	}

	var intent types.Intent
	if err := json.Unmarshal([]byte(val), &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent: %w", err)
	}

	return &intent, nil
}

// Close closes the redis client
func (m *RedisAgentMemory) Close() error {
	return m.client.Close()
}
