package rateLimiter

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Storage defines the interface for rate limit storage backends.
// Implementations must be safe for concurrent use.
type Storage interface {
	// GetBucket retrieves the current state of a rate limit bucket.
	// It returns the number of tokens available, the last update time,
	// and any error that occurred during retrieval.
	GetBucket(ctx context.Context, key string) (tokens float64, lastUpdate time.Time, err error)

	// UpdateBucket updates the state of a rate limit bucket.
	// It sets the number of tokens and expiry time for the bucket.
	UpdateBucket(ctx context.Context, key string, tokens float64, expiry time.Duration) error
}

// RedisStorage implements the Storage interface using Redis as the backend.
// It provides distributed rate limiting capabilities suitable for multi-instance deployments.
type RedisStorage struct {
	client *redis.Client
}

// InMemoryStorage implements the Storage interface using an in-memory map.
// It provides a simple, fast storage backend suitable for single-instance deployments
// or as a fallback when Redis is unavailable.
type InMemoryStorage struct {
	buckets map[string]*bucketState
	mutex   sync.RWMutex
}

// bucketState represents the current state of a rate limit bucket in memory.
type bucketState struct {
	tokens     float64
	lastUpdate time.Time
	expiry     time.Time
}

// NewRedisStorage creates a new Redis-based storage backend.
// The provided Redis client must be properly configured and connected.
func NewRedisStorage(client *redis.Client) *RedisStorage {
	return &RedisStorage{client: client}
}

// NewInMemoryStorage creates a new in-memory storage backend.
// This implementation is thread-safe and suitable for single-instance deployments.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		buckets: make(map[string]*bucketState),
	}
}

// GetBucket retrieves the current state of a rate limit bucket from memory.
// If the bucket doesn't exist or has expired, it returns default values.
func (ims *InMemoryStorage) GetBucket(ctx context.Context, key string) (float64, time.Time, error) {
	ims.mutex.RLock()
	defer ims.mutex.RUnlock()

	bucket, exists := ims.buckets[key]
	if !exists || time.Now().After(bucket.expiry) {
		return 0, time.Now(), nil
	}

	return bucket.tokens, bucket.lastUpdate, nil
}

// UpdateBucket updates the state of a rate limit bucket in memory.
// It also performs periodic cleanup of expired entries when the bucket count exceeds 10000.
func (ims *InMemoryStorage) UpdateBucket(ctx context.Context, key string, tokens float64, expiry time.Duration) error {
	ims.mutex.Lock()
	defer ims.mutex.Unlock()

	// Clean expired entries periodically (simple implementation)
	if len(ims.buckets) > 10000 { // Arbitrary threshold for cleanup
		now := time.Now()
		for k, v := range ims.buckets {
			if now.After(v.expiry) {
				delete(ims.buckets, k)
			}
		}
	}

	ims.buckets[key] = &bucketState{
		tokens:     tokens,
		lastUpdate: time.Now(),
		expiry:     time.Now().Add(expiry),
	}
	return nil
}

// GetBucket retrieves the current state of a rate limit bucket from Redis.
// If the bucket doesn't exist or is incomplete, it returns default values.
func (rs *RedisStorage) GetBucket(ctx context.Context, key string) (float64, time.Time, error) {
	// Get bucket data from Redis
	data, err := rs.client.HGetAll(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return 0, time.Time{}, err
	}

	// If key doesn't exist or is incomplete, return default values
	if len(data) == 0 || data["tokens"] == "" || data["lastUpdate"] == "" {
		return 0, time.Now(), nil
	}

	// Parse data
	tokens, err := strconv.ParseFloat(data["tokens"], 64)
	if err != nil {
		return 0, time.Time{}, err
	}

	lastUpdateUnix, err := strconv.ParseInt(data["lastUpdate"], 10, 64)
	if err != nil {
		return 0, time.Time{}, err
	}

	lastUpdate := time.Unix(0, lastUpdateUnix)
	return tokens, lastUpdate, nil
}

// UpdateBucket updates the state of a rate limit bucket in Redis.
// It uses a Redis transaction to ensure atomic updates of the bucket state.
func (rs *RedisStorage) UpdateBucket(ctx context.Context, key string, tokens float64, expiry time.Duration) error {
	// Update bucket data in Redis
	now := time.Now()
	_, err := rs.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, "tokens", tokens)
		pipe.HSet(ctx, key, "lastUpdate", now.UnixNano())
		pipe.Expire(ctx, key, expiry)
		return nil
	})
	return err
}
