# Rate Limiter

A flexible and robust rate limiting middleware for Go applications using the Fiber web framework. This rate limiter supports both HTTP and WebSocket connections, with configurable policies and multiple storage backends.

## Features

- HTTP and WebSocket rate limiting
- Configurable rate limiting policies per user tier
- Redis and in-memory storage support
- Token bucket algorithm for burst handling
- Customizable user identification and tier assignment
- Path-based rate limiting exclusions
- Fallback storage mechanism

## Installation

```bash

go get github.com/Popoola-Opeyemi/rateLimiter
```

## Quick Start

Here's a basic example of how to use the rate limiter in your Fiber application:

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/redis/go-redis/v9"
    "github.com/yourusername/rateLimiter"
    "time"
)

func main() {
    app := fiber.New()

    // Configure Redis client (optional)
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    // Define rate limiting policies
    policies := map[string]rateLimiter.Policy{
        "free": {
            MaxRequests:     100,
            BurstCapacity:  10,
            TokensPerSecond: 0.1,
            WebSocketAllowed: false,
        },
        "premium": {
            MaxRequests:     1000,
            BurstCapacity:  50,
            TokensPerSecond: 0.5,
            WebSocketAllowed: true,
        },
    }

    // Default policy for unknown tiers
    defaultPolicy := rateLimiter.Policy{
        MaxRequests:     50,
        BurstCapacity:  5,
        TokensPerSecond: 0.05,
        WebSocketAllowed: false,
    }

    // Configure rate limiter
    config := rateLimiter.RateLimiterConfig{
        Redis:         redisClient,
        TierPolicy:    policies,
        DefaultPolicy: defaultPolicy,
        KeyPrefix:     "ratelimit:",
        GetUserID: func(c *fiber.Ctx) string {
            // Implement your user ID extraction logic
            return c.Get("X-User-ID")
        },
        GetUserTier: func(c *fiber.Ctx) string {
            // Implement your tier determination logic
            return c.Get("X-User-Tier")
        },
        SkipPaths: []string{"/health", "/metrics"},
    }

    // Apply rate limiter middleware
    app.Use(rateLimiter.RateLimiter(config))

    // Your routes here
    app.Get("/", func(c *fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })

    app.Listen(":3000")
}
```

## Configuration

### RateLimiterConfig

The rate limiter can be configured using the `RateLimiterConfig` struct:

```go
type RateLimiterConfig struct {
    Redis         *redis.Client    // Redis client for distributed rate limiting
    TierPolicy    map[string]Policy // Rate limiting policies per user tier
    DefaultPolicy Policy           // Default policy for unknown tiers
    KeyPrefix     string           // Prefix for rate limit keys in storage
    GetUserID     func(c *fiber.Ctx) string    // Function to extract user ID
    GetUserTier   func(c *fiber.Ctx) string    // Function to determine user tier
    SkipPaths     []string         // Paths to exclude from rate limiting
}
```

### Policy

Each policy defines the rate limiting rules for a specific tier:

```go
type Policy struct {
    MaxRequests      int           // Maximum requests allowed in the window
    BurstCapacity    int           // Maximum burst capacity
    TokensPerSecond  float64       // Token refill rate
    WebSocketAllowed bool          // Whether WebSockets are allowed
}
```

## Storage Backends

The rate limiter supports two storage backends:

1. **Redis Storage**: For distributed rate limiting across multiple instances
2. **In-Memory Storage**: As a fallback or for single-instance applications

The rate limiter automatically uses Redis if configured, falling back to in-memory storage if Redis is unavailable.

## Rate Limiting Headers

The middleware adds the following headers to responses:

- `X-RateLimit-Limit`: Maximum requests allowed in the window
- `X-RateLimit-Remaining`: Remaining requests in the current window
- `X-RateLimit-Reset`: Time until the rate limit resets (Unix timestamp)

## Error Handling

When a rate limit is exceeded, the middleware returns a `429 Too Many Requests` response with a JSON body:

```json
{
    "error": "rate limit exceeded",
    "retry_after": 3600
}
```

## Best Practices

1. Always configure a reasonable `DefaultPolicy` for unknown user tiers
2. Use Redis storage in production environments with multiple instances
3. Implement proper user identification and tier determination logic
4. Monitor rate limit headers to track usage patterns
5. Configure appropriate burst capacities based on your application's needs

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
