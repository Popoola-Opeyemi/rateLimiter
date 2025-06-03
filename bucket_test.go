package rateLimiter

import (
	"context"
	"testing"
	"time"
)

func TestCheckTokenBucket(t *testing.T) {
	ctx := context.Background()
	primaryStorage := NewInMemoryStorage()
	fallbackStorage := NewInMemoryStorage()

	// Test policy with 2 tokens per second and burst capacity of 5
	policy := Policy{
		MaxRequests:      100,
		BurstCapacity:    5,
		TokensPerSecond:  2.0,
		WebSocketAllowed: true,
	}

	t.Run("New bucket initialization", func(t *testing.T) {
		allowed, retryAfter, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, "test1", policy)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !allowed {
			t.Error("Expected request to be allowed for new bucket")
		}
		if retryAfter != 0 {
			t.Errorf("Expected retryAfter to be 0, got %d", retryAfter)
		}
	})

	t.Run("Burst capacity", func(t *testing.T) {
		// Should be able to make 5 requests immediately (burst capacity)
		for i := 0; i < 5; i++ {
			allowed, _, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, "test2", policy)
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if !allowed {
				t.Errorf("Expected request %d to be allowed", i+1)
			}
		}

		// Next request should be rate limited
		allowed, retryAfter, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, "test2", policy)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if allowed {
			t.Error("Expected request to be rate limited")
		}
		if retryAfter < 1 {
			t.Errorf("Expected retryAfter to be at least 1, got %d", retryAfter)
		}
	})

	t.Run("Token refill", func(t *testing.T) {
		// Use up all tokens
		for i := 0; i < 5; i++ {
			_, _, _ = checkTokenBucket(ctx, primaryStorage, fallbackStorage, "test3", policy)
		}

		// Wait for 1 token to refill (0.5 seconds)
		time.Sleep(500 * time.Millisecond)

		// Should be able to make one more request
		allowed, retryAfter, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, "test3", policy)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !allowed {
			t.Error("Expected request to be allowed after token refill")
		}
		if retryAfter != 0 {
			t.Errorf("Expected retryAfter to be 0, got %d", retryAfter)
		}
	})

	t.Run("Storage fallback", func(t *testing.T) {
		// Create a failing primary storage
		failingStorage := &FailingStorage{}

		// Should fall back to in-memory storage
		allowed, retryAfter, err := checkTokenBucket(ctx, failingStorage, fallbackStorage, "test4", policy)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !allowed {
			t.Error("Expected request to be allowed with fallback storage")
		}
		if retryAfter != 0 {
			t.Errorf("Expected retryAfter to be 0, got %d", retryAfter)
		}
	})

	t.Run("Different keys isolation", func(t *testing.T) {
		// Use up all tokens for key1
		for i := 0; i < 5; i++ {
			_, _, _ = checkTokenBucket(ctx, primaryStorage, fallbackStorage, "key1", policy)
		}

		// Should still be able to make requests with key2
		allowed, retryAfter, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, "key2", policy)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !allowed {
			t.Error("Expected request to be allowed for different key")
		}
		if retryAfter != 0 {
			t.Errorf("Expected retryAfter to be 0, got %d", retryAfter)
		}
	})
}

// FailingStorage is a test implementation that always fails
type FailingStorage struct{}

func (fs *FailingStorage) GetBucket(ctx context.Context, key string) (float64, time.Time, error) {
	return 0, time.Time{}, &StorageError{Message: "simulated failure"}
}

func (fs *FailingStorage) UpdateBucket(ctx context.Context, key string, tokens float64, expiry time.Duration) error {
	return &StorageError{Message: "simulated failure"}
}

// StorageError is a custom error type for storage operations
type StorageError struct {
	Message string
}

func (e *StorageError) Error() string {
	return e.Message
}
