package rateLimiter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// checkSecurity performs security-related checks before rate limiting
func checkSecurity(c *fiber.Ctx, cfg RateLimiterConfig) (bool, error) {
	// Check for bypass token
	if bypassToken := c.Get("X-RateLimit-Bypass"); bypassToken != "" {
		if cfg.GlobalSecurity.ValidateBypassToken(bypassToken) {
			return true, nil
		}
	}

	// Check IP whitelist
	ip := c.IP()
	if cfg.GlobalSecurity.IsIPWhitelisted(ip) {
		return true, nil
	}

	// Check if IP is blocked due to too many failed attempts
	if isBlocked, err := checkIPBlocked(c, cfg); err != nil {
		return false, err
	} else if isBlocked {
		return false, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "IP temporarily blocked due to too many failed attempts",
		})
	}

	return false, nil
}

// checkIPBlocked checks if an IP is blocked due to too many failed attempts
func checkIPBlocked(c *fiber.Ctx, cfg RateLimiterConfig) (bool, error) {
	ip := c.IP()
	blockKey := fmt.Sprintf("%s:blocked:%s", cfg.KeyPrefix, ip)
	failedKey := fmt.Sprintf("%s:failed:%s", cfg.KeyPrefix, ip)

	if cfg.Redis != nil {
		ctx := context.Background()
		pipe := cfg.Redis.Pipeline()

		// Get both block status and failed attempts
		pipe.Get(ctx, blockKey)
		pipe.Get(ctx, failedKey)

		results, err := pipe.Exec(ctx)
		if err != nil && err != redis.Nil {
			return false, err
		}

		// Check if IP is blocked
		blocked, _ := results[0].(*redis.StringCmd).Bool()
		if blocked {
			// Get remaining block time
			ttl, err := cfg.Redis.TTL(ctx, blockKey).Result()
			if err != nil {
				return true, nil
			}

			// Return block status with remaining time
			return true, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":           "IP temporarily blocked due to too many failed attempts",
				"retry_after":     int(ttl.Seconds()),
				"block_remaining": ttl.String(),
			})
		}

		// Check if we should apply progressive blocking
		failedAttempts, _ := results[1].(*redis.StringCmd).Int64()
		if failedAttempts > 0 {
			// Apply temporary slowdown for IPs with failed attempts
			// but not yet blocked
			slowdownDuration := time.Duration(failedAttempts) * 5 * time.Second
			if slowdownDuration > 30*time.Second {
				slowdownDuration = 30 * time.Second
			}

			return false, c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":           "Too many failed attempts, please wait before trying again",
				"retry_after":     int(slowdownDuration.Seconds()),
				"failed_attempts": failedAttempts,
			})
		}
	}

	return false, nil
}

// recordFailedAttempt records a failed attempt and blocks the IP if necessary
func recordFailedAttempt(c *fiber.Ctx, cfg RateLimiterConfig) error {
	ip := c.IP()
	failedKey := fmt.Sprintf("%s:failed:%s", cfg.KeyPrefix, ip)
	blockKey := fmt.Sprintf("%s:blocked:%s", cfg.KeyPrefix, ip)

	if cfg.Redis != nil {
		ctx := context.Background()
		pipe := cfg.Redis.Pipeline()

		// Increment failed attempts
		pipe.Incr(ctx, failedKey)
		pipe.Expire(ctx, failedKey, 24*time.Hour)

		// Check if we should block the IP
		pipe.Get(ctx, failedKey)

		results, err := pipe.Exec(ctx)
		if err != nil {
			return err
		}

		failedAttempts := results[1].(*redis.IntCmd).Val()
		if failedAttempts >= int64(cfg.GlobalSecurity.MaxFailedAttempts) {
			// Calculate progressive block duration
			// Each additional failed attempt increases block time
			baseDuration := cfg.GlobalSecurity.BlockDuration
			extraAttempts := failedAttempts - int64(cfg.GlobalSecurity.MaxFailedAttempts)
			blockDuration := baseDuration + time.Duration(extraAttempts)*5*time.Minute

			// Cap maximum block duration at 24 hours
			if blockDuration > 24*time.Hour {
				blockDuration = 24 * time.Hour
			}

			// Block the IP with progressive duration
			cfg.Redis.Set(ctx, blockKey, true, blockDuration)

			// Log the blocking event
			fmt.Printf("IP %s blocked for %v due to %d failed attempts\n",
				ip, blockDuration, failedAttempts)
		}
	}

	return nil
}

func HandleWebSocketUpgrade(c *fiber.Ctx, primaryStorage, fallbackStorage Storage, cfg RateLimiterConfig) error {
	// Check security first
	if bypass, err := checkSecurity(c, cfg); err != nil {
		return err
	} else if bypass {
		return c.Next()
	}

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
	// Check security first
	if bypass, err := checkSecurity(c, cfg); err != nil {
		return err
	} else if bypass {
		return c.Next()
	}

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

	// Check authentication requirement
	if policy.Security.RequireAuthentication && identifier == c.IP() {
		// Apply stricter rate limiting for unauthenticated requests
		policy.TokensPerSecond = policy.TokensPerSecond * 0.5
		policy.BurstCapacity = policy.BurstCapacity / 2
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
		// Record failed attempt if this is an authentication endpoint
		if strings.Contains(endpoint, "auth") || strings.Contains(endpoint, "login") {
			if err := recordFailedAttempt(c, cfg); err != nil {
				// Log error but continue with rate limit response
				fmt.Printf("Error recording failed attempt: %v\n", err)
			}
		}

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
