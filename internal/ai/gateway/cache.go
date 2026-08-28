package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultCacheTTL  = 60 * time.Second
	CacheKeyPrefix   = "cache:resp:"
)

// ResponseCache provides Redis-backed response caching for non-streaming requests.
// Cache keys are scoped per tenant and hashed from the request payload.
type ResponseCache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewResponseCache(rdb *redis.Client, ttl time.Duration) *ResponseCache {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &ResponseCache{rdb: rdb, ttl: ttl}
}

// cachePayload is the subset of request fields used to generate the cache key.
// Only semantically meaningful fields are included (not stream, user, etc.).
type cachePayload struct {
	Model            string          `json:"model"`
	Messages         []Message       `json:"messages"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	N                *int            `json:"n,omitempty"`
	Stop             any             `json:"stop,omitempty"`
	ResponseFormat   *ResponseFormat `json:"response_format,omitempty"`
	Tools            []Tool          `json:"tools,omitempty"`
	ToolChoice       any             `json:"tool_choice,omitempty"`
	ReasoningEffort  string          `json:"reasoning_effort,omitempty"`
}

// GenerateKey creates a cache key from tenant ID and request.
// Key format: cache:resp:<tenantID>:<sha256(payload)>
func GenerateKey(tenantID string, req *ChatRequest) string {
	payload := cachePayload{
		Model:            req.Model,
		Messages:         req.Messages,
		Temperature:      req.Temperature,
		MaxTokens:        req.MaxTokens,
		TopP:             req.TopP,
		FrequencyPenalty: req.FrequencyPenalty,
		PresencePenalty:  req.PresencePenalty,
		N:                req.N,
		Stop:             req.Stop,
		ResponseFormat:   req.ResponseFormat,
		Tools:            req.Tools,
		ToolChoice:       req.ToolChoice,
		ReasoningEffort:  req.ReasoningEffort,
	}

	data, _ := json.Marshal(payload)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%s%s:%x", CacheKeyPrefix, tenantID, hash)
}

// Get retrieves a cached response. Returns nil on miss or error.
func (c *ResponseCache) Get(ctx context.Context, key string) *ChatResponse {
	if c.rdb == nil {
		return nil
	}

	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}

	var resp ChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	return &resp
}

// Set stores a response in the cache.
func (c *ResponseCache) Set(ctx context.Context, key string, resp *ChatResponse) {
	if c.rdb == nil || resp == nil {
		return
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	c.rdb.Set(ctx, key, data, c.ttl)
}

// InvalidateByTenant removes all cached responses for a tenant.
// Returns the number of keys deleted.
func (c *ResponseCache) InvalidateByTenant(ctx context.Context, tenantID string) (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	pattern := fmt.Sprintf("%s%s:*", CacheKeyPrefix, tenantID)
	return c.deleteByPattern(ctx, pattern)
}

// InvalidateAll removes all cached responses.
// Returns the number of keys deleted.
func (c *ResponseCache) InvalidateAll(ctx context.Context) (int64, error) {
	if c.rdb == nil {
		return 0, nil
	}
	pattern := CacheKeyPrefix + "*"
	return c.deleteByPattern(ctx, pattern)
}

// InvalidateKey removes a specific cached response.
func (c *ResponseCache) InvalidateKey(ctx context.Context, key string) error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, key).Err()
}

// deleteByPattern scans and deletes keys matching a pattern.
func (c *ResponseCache) deleteByPattern(ctx context.Context, pattern string) (int64, error) {
	var total int64
	var cursor uint64
	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return total, err
		}
		if len(keys) > 0 {
			n, err := c.rdb.Del(ctx, keys...).Result()
			if err != nil {
				return total, err
			}
			total += n
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return total, nil
}
