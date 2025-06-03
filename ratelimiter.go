// Package rateLimiter provides a flexible and robust rate limiting middleware for Go applications
// using the Fiber web framework. It supports both HTTP and WebSocket connections with configurable
// policies and multiple storage backends.
//
// The package implements a token bucket algorithm for rate limiting, which allows for handling
// traffic bursts while maintaining overall rate limits. It supports both Redis and in-memory
// storage backends, with automatic fallback to in-memory storage when Redis is unavailable.
package rateLimiter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// RateLimiter creates a new rate limiting middleware for Fiber applications.
// It takes a RateLimiterConfig struct that defines the rate limiting behavior
// and returns a Fiber middleware handler.
//
// The middleware automatically handles both HTTP and WebSocket requests, applying
// the appropriate rate limiting policies based on the user's tier. It supports
// path-based exclusions and provides detailed rate limit information in response headers.
//
// Example:
//
//	app := fiber.New()
//	config := RateLimiterConfig{
//		Redis: redisClient,
//		TierPolicy: map[string]Policy{
//			"free": {
//				MaxRequests:      1000,        // Allow up to 1000 requests per window
//				BurstCapacity:    50,          // Allow bursts of up to 50 requests
//				TokensPerSecond:  1.0,         // Allow 1 request per second in steady state
//				WebSocketAllowed: false,
//			},
//			"pro": {
//				MaxRequests:      5000,        // Allow up to 5000 requests per window
//				BurstCapacity:    200,         // Allow bursts of up to 200 requests
//				TokensPerSecond:  5.0,         // Allow 5 requests per second in steady state
//				WebSocketAllowed: true,
//			},
//		},
//		DefaultPolicy: Policy{
//			MaxRequests:      500,           // Default to 500 requests per window
//			BurstCapacity:    25,            // Default burst of 25 requests
//			TokensPerSecond:  0.5,           // Default to 0.5 requests per second
//			WebSocketAllowed: false,
//		},
//		KeyPrefix: "rl",
//		GetUserID: func(c *fiber.Ctx) string {
//			return c.Get("X-User-ID")
//		},
//		GetUserTier: func(c *fiber.Ctx) string {
//			return c.Get("X-User-Tier")
//		},
//		SkipPaths: []string{"/metrics", "/health"},
//	}
//	app.Use(RateLimiter(config))
func RateLimiter(cfg RateLimiterConfig) fiber.Handler {
	var primaryStorage Storage
	var fallbackStorage Storage = NewInMemoryStorage()

	// Initialize primary storage
	if cfg.Redis != nil {
		primaryStorage = NewRedisStorage(cfg.Redis)
	} else {
		primaryStorage = fallbackStorage
	}

	return func(c *fiber.Ctx) error {
		// Check if path should be skipped
		for _, path := range cfg.SkipPaths {
			if c.Path() == path {
				return c.Next()
			}
		}

		// Special handling for WebSocket upgrade requests
		if websocket.IsWebSocketUpgrade(c) {
			return HandleWebSocketUpgrade(c, primaryStorage, fallbackStorage, cfg)
		}

		return HandleHTTPRequest(c, primaryStorage, fallbackStorage, cfg)
	}
}
