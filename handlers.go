package rateLimiter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

func HandleWebSocketUpgrade(c *fiber.Ctx, primaryStorage, fallbackStorage Storage, cfg RateLimiterConfig) error {
	ctx := context.Background()

	// Identify user and tier
	identifier := cfg.GetUserID(c)
	if identifier == "" {
		identifier = c.IP()
	}

	tier := cfg.GetUserTier(c)
	if tier == "" {
		tier = "free"
	}

	// Get policy for this tier
	policy, ok := cfg.TierPolicy[tier]
	if !ok {
		policy = cfg.DefaultPolicy
	}

	// Check if WebSockets are allowed for this tier
	if !policy.WebSocketAllowed {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "WebSocket connections not allowed for your tier",
			"tier":  tier,
		})
	}

	// Special key for WebSocket connections (usually more expensive)
	endpoint := strings.ReplaceAll(strings.Trim(c.Route().Path, "/"), "/", "_")
	key := fmt.Sprintf("%s:%s:%s:ws", cfg.KeyPrefix, identifier, endpoint)

	// Use token bucket algorithm for WebSocket rate limiting
	allow, retryAfter, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, key, policy)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal rate limit error",
		})
	}

	if !allow {
		c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "rate limit exceeded for WebSocket connection",
			"retry_after": retryAfter,
			"tier":        tier,
		})
	}

	return c.Next()
}

func HandleHTTPRequest(c *fiber.Ctx, primaryStorage, fallbackStorage Storage, cfg RateLimiterConfig) error {
	ctx := context.Background()

	// Identify user and tier
	identifier := cfg.GetUserID(c)
	if identifier == "" {
		identifier = c.IP()
	}

	tier := cfg.GetUserTier(c)
	if tier == "" {
		tier = "free"
	}

	// Get policy for this tier
	policy, ok := cfg.TierPolicy[tier]
	if !ok {
		policy = cfg.DefaultPolicy
	}

	// Create unique key based on the endpoint access
	endpoint := strings.ReplaceAll(strings.Trim(c.Route().Path, "/"), "/", "_")
	key := fmt.Sprintf("%s:%s:%s", cfg.KeyPrefix, identifier, endpoint)

	// Use token bucket algorithm
	allow, retryAfter, err := checkTokenBucket(ctx, primaryStorage, fallbackStorage, key, policy)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal rate limit error",
		})
	}

	// Set rate limit headers
	c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", policy.MaxRequests))
	c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int(policy.BurstCapacity)))
	c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", int(time.Now().Add(time.Second*time.Duration(retryAfter)).Unix())))

	if !allow {
		// Add Retry-After header (RFC 7231, Section 7.1.3)
		c.Set("Retry-After", fmt.Sprintf("%d", retryAfter))

		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "rate limit exceeded",
			"limit":       policy.MaxRequests,
			"retry_after": retryAfter,
			"tier":        tier,
		})
	}

	return c.Next()
}
