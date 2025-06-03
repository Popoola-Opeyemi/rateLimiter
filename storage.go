package rateLimiter

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Storage interface {
	GetBucket(ctx context.Context, key string) (tokens float64, lastUpdate time.Time, err error)
	UpdateBucket(ctx context.Context, key string, tokens float64, expiry time.Duration) error
}

type RedisStorage struct {
	client *redis.Client
}

type InMemoryStorage struct {
	buckets map[string]*bucketState
	mutex   sync.RWMutex
}

type bucketState struct {
	tokens     float64
	lastUpdate time.Time
	expiry     time.Time
}

func NewRedisStorage(client *redis.Client) *RedisStorage {
	return &RedisStorage{client: client}
}

func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		buckets: make(map[string]*bucketState),
	}
}

func (ims *InMemoryStorage) GetBucket(ctx context.Context, key string) (float64, time.Time, error) {
	ims.mutex.RLock()
	defer ims.mutex.RUnlock()

	bucket, exists := ims.buckets[key]
	if !exists || time.Now().After(bucket.expiry) {
		return 0, time.Now(), nil
	}

	return bucket.tokens, bucket.lastUpdate, nil
}

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
