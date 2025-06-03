package rateLimiter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// Policy defines the rate limiting rules for a specific user tier.
// It implements a token bucket algorithm for rate limiting, which allows
// for handling traffic bursts while maintaining overall rate limits.
type Policy struct {
	// MaxRequests is the maximum number of requests allowed in the time window.
	MaxRequests int

	// BurstCapacity is the maximum number of tokens that can be accumulated in the bucket.
	// This determines how many requests can be made in a burst before rate limiting kicks in.
	BurstCapacity int

	// TokensPerSecond is the rate at which tokens are added to the bucket.
	// For example, a value of 0.1 means one token is added every 10 seconds.
	TokensPerSecond float64

	// WebSocketAllowed determines whether WebSocket connections are allowed for this tier.
	// If false, WebSocket upgrade requests will be rejected.
	WebSocketAllowed bool
}

// RateLimiterConfig defines the configuration for the rate limiter middleware.
// It specifies how rate limiting should be applied to different user tiers and
// how user identification should be handled.
type RateLimiterConfig struct {
	// Redis is the Redis client used for distributed rate limiting.
	// If nil, the rate limiter will use in-memory storage.
	Redis *redis.Client

	// TierPolicy maps user tiers to their respective rate limiting policies.
	// Each tier can have its own set of rate limiting rules.
	TierPolicy map[string]Policy

	// DefaultPolicy is applied when a user's tier is not found in TierPolicy.
	// This ensures that unknown tiers still have rate limiting applied.
	DefaultPolicy Policy

	// KeyPrefix is the prefix used for rate limit keys in storage.
	// This helps prevent key collisions when multiple applications use the same storage.
	KeyPrefix string

	// GetUserID is a function that extracts the user ID from the request context.
	// This function should return a unique identifier for the user making the request.
	GetUserID func(c *fiber.Ctx) string

	// GetUserTier is a function that determines the user's tier from the request context.
	// This function should return the tier name that corresponds to a policy in TierPolicy.
	GetUserTier func(c *fiber.Ctx) string

	// SkipPaths is a list of paths that should be excluded from rate limiting.
	// Requests to these paths will bypass the rate limiter completely.
	SkipPaths []string
}
