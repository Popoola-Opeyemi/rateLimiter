package rateLimiter

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// SecurityConfig defines security-related settings for rate limiting
type SecurityConfig struct {
	// BypassTokens is a list of valid bypass tokens that can skip rate limiting
	// These should be secure, randomly generated tokens
	BypassTokens []string

	// WhitelistIPs is a list of IP addresses that are exempt from rate limiting
	WhitelistIPs []string

	// RequireAuthentication determines if rate limiting should be stricter for unauthenticated requests
	RequireAuthentication bool

	// MaxFailedAttempts is the maximum number of failed attempts before stricter rate limiting
	MaxFailedAttempts int

	// BlockDuration is how long to block an IP after exceeding MaxFailedAttempts
	BlockDuration time.Duration
}

// Policy defines the rate limiting rules for a specific user tier.
// It implements a token bucket algorithm for rate limiting, which allows
// for handling traffic bursts while maintaining overall rate limits.
type Policy struct {
	// MaxRequests is the maximum number of requests allowed in the time window.
	// This is a soft limit that works in conjunction with the token bucket algorithm.
	// The actual rate limiting is primarily controlled by BurstCapacity and TokensPerSecond.
	MaxRequests int

	// BurstCapacity is the maximum number of tokens that can be accumulated in the bucket.
	// This determines how many requests can be made in a burst before rate limiting kicks in.
	// For example, a value of 50 means users can make up to 50 requests in quick succession
	// before the steady-state rate (TokensPerSecond) is enforced.
	BurstCapacity int

	// TokensPerSecond is the rate at which tokens are added to the bucket.
	// This defines the steady-state rate of requests after the burst capacity is exhausted.
	// For example:
	// - 1.0 means one request per second
	// - 0.1 means one request every 10 seconds
	// - 5.0 means five requests per second
	TokensPerSecond float64

	// WebSocketAllowed determines whether WebSocket connections are allowed for this tier.
	// If false, WebSocket upgrade requests will be rejected with a 403 Forbidden response.
	// WebSocket connections are typically more resource-intensive and may need stricter controls.
	WebSocketAllowed bool

	// Security contains security-related settings for this policy
	Security SecurityConfig
}

// RateLimiterConfig defines the configuration for the rate limiter middleware.
// It specifies how rate limiting should be applied to different user tiers and
// how user identification should be handled.
type RateLimiterConfig struct {
	// Redis is the Redis client used for distributed rate limiting.
	// If nil, the rate limiter will use in-memory storage.
	// In-memory storage is suitable for single-instance applications,
	// while Redis is recommended for distributed deployments.
	Redis *redis.Client

	// TierPolicy maps user tiers to their respective rate limiting policies.
	// Each tier can have its own set of rate limiting rules.
	// Common tiers might include "free", "pro", "enterprise", etc.
	// If a user's tier is not found here, the DefaultPolicy will be used.
	TierPolicy map[string]Policy

	// DefaultPolicy is applied when a user's tier is not found in TierPolicy.
	// This ensures that unknown tiers still have rate limiting applied.
	// It's recommended to set this to a conservative policy that protects
	// your API from abuse by unknown users.
	DefaultPolicy Policy

	// KeyPrefix is the prefix used for rate limit keys in storage.
	// This helps prevent key collisions when multiple applications use the same storage.
	// For example, if KeyPrefix is "rl", the keys will be stored as "rl:user_id:endpoint".
	KeyPrefix string

	// GetUserID is a function that extracts the user ID from the request context.
	// This function should return a unique identifier for the user making the request.
	// If it returns an empty string, the client's IP address will be used as the identifier.
	GetUserID func(c *fiber.Ctx) string

	// GetUserTier is a function that determines the user's tier from the request context.
	// This function should return the tier name that corresponds to a policy in TierPolicy.
	// If it returns an empty string, the user will be treated as a "free" tier user.
	GetUserTier func(c *fiber.Ctx) string

	// SkipPaths is a list of paths that should be excluded from rate limiting.
	// Requests to these paths will bypass the rate limiter completely.
	// This is useful for health checks, metrics endpoints, or other system paths
	// that should not be rate limited.
	SkipPaths []string

	// GlobalSecurity contains security settings that apply to all requests
	GlobalSecurity SecurityConfig
}

// ValidateBypassToken checks if a token is valid and returns true if it is
func (sc *SecurityConfig) ValidateBypassToken(token string) bool {
	if token == "" {
		return false
	}

	// Hash the token for comparison to prevent timing attacks
	hashedToken := sha256.Sum256([]byte(token))
	hashedTokenStr := hex.EncodeToString(hashedToken[:])

	for _, validToken := range sc.BypassTokens {
		hashedValidToken := sha256.Sum256([]byte(validToken))
		hashedValidTokenStr := hex.EncodeToString(hashedValidToken[:])
		if hashedTokenStr == hashedValidTokenStr {
			return true
		}
	}
	return false
}

// IsIPWhitelisted checks if an IP address is in the whitelist
func (sc *SecurityConfig) IsIPWhitelisted(ip string) bool {
	for _, whitelistedIP := range sc.WhitelistIPs {
		if ip == whitelistedIP {
			return true
		}
	}
	return false
}
