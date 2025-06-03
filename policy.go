package rateLimiter

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

type Policy struct {
	MaxRequests      int           // Maximum number of requests allowed in the window
	Window           time.Duration // Time window for rate limiting
	BurstCapacity    int           // Maximum burst capacity (token bucket max tokens)
	TokensPerSecond  float64       // Token refill rate per second (token bucket)
	WebSocketAllowed bool          // Whether WebSockets are allowed for this tier
}

type RateLimiterConfig struct {
	Redis         *redis.Client
	TierPolicy    map[string]Policy
	DefaultPolicy Policy
	KeyPrefix     string
	GetUserID     func(c *fiber.Ctx) string
	GetUserTier   func(c *fiber.Ctx) string
	SkipPaths     []string // Paths to skip rate limiting
}
