package rateLimiter

import (
	"context"
	"math"
	"time"
)

// checkTokenBucket implements the token bucket algorithm for rate limiting.
// It manages a bucket of tokens that are consumed by requests and refilled over time.
//
// The function takes a key (typically user ID or IP), a policy defining the rate limits,
// and both primary and fallback storage backends. It returns:
//   - bool: whether the request should be allowed
//   - int: seconds to wait if the request is rejected
//   - error: any error that occurred during the check
//
// The token bucket algorithm works as follows:
//  1. Each request consumes one token
//  2. Tokens are refilled at a constant rate (TokensPerSecond)
//  3. The bucket has a maximum capacity (BurstCapacity)
//  4. If the bucket is empty, requests are rejected
func checkTokenBucket(ctx context.Context, primaryStorage, fallbackStorage Storage,
	key string, policy Policy) (bool, int, error) {

	var (
		tokens     float64
		lastUpdate time.Time
		err        error
		now        = time.Now()
	)

	// Try to get bucket from primary storage
	tokens, lastUpdate, err = primaryStorage.GetBucket(ctx, key)
	if err != nil {
		tokens, lastUpdate, err = fallbackStorage.GetBucket(ctx, key)
		if err != nil {
			return false, 0, err
		}
	}

	// Time to refill the entire bucket (used as TTL)
	ttl := time.Duration(float64(policy.BurstCapacity)/policy.TokensPerSecond) * time.Second

	// If bucket is uninitialized
	if lastUpdate.IsZero() {
		tokens = float64(policy.BurstCapacity)
		lastUpdate = now
		updateBothStorages(ctx, key, tokens, ttl, primaryStorage, fallbackStorage)
	} else {
		// Calculate elapsed time and refill tokens
		elapsed := now.Sub(lastUpdate).Seconds()
		refilled := elapsed * policy.TokensPerSecond
		tokens = min(float64(policy.BurstCapacity), tokens+refilled)
	}

	// Not enough tokens to allow request
	if tokens < 1 {
		tokensNeeded := 1 - tokens
		secondsToWait := int(math.Ceil(tokensNeeded / policy.TokensPerSecond))

		// Ensure at least 1 second wait time
		if secondsToWait < 1 {
			secondsToWait = 1
		}

		updateBothStorages(ctx, key, tokens, ttl, primaryStorage, fallbackStorage)
		return false, secondsToWait, nil
	}

	// Consume one token
	tokens--

	updateBothStorages(ctx, key, tokens, ttl, primaryStorage, fallbackStorage)
	return true, 0, nil
}

// updateBothStorages updates the bucket state in both primary and fallback storage.
// This ensures consistency between storage backends and provides redundancy.
// If an error occurs during update, it is currently logged but not returned.
func updateBothStorages(ctx context.Context, key string, tokens float64,
	ttl time.Duration, primary, fallback Storage) {
	if err := primary.UpdateBucket(ctx, key, tokens, ttl); err != nil {
		// TODO: Add logging
	}
	if err := fallback.UpdateBucket(ctx, key, tokens, ttl); err != nil {
		// TODO: Add logging
	}
}
