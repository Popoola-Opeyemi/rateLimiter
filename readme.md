# Token Bucket Rate Limiter for Fiber

This package provides a robust, feature-rich rate limiting middleware for [Fiber](https://github.com/gofiber/fiber) web applications using the token bucket algorithm.

## Features

- **Token Bucket Algorithm**: Smooth handling of traffic bursts with configurable bucket size and refill rates
- **Storage Abstraction**: Primary Redis storage with automatic fallback to in-memory when Redis is unavailable
- **WebSocket Support**: Special handling for WebSocket connections with dedicated WebSocket policies
- **Retry Headers**: Standards-compliant `Retry-After` headers and comprehensive rate limit information
- **Tiered Rate Limiting**: Different rate limit policies for different user tiers
- **Configurable Identifiers**: Flexible identification based on user ID, IP address, or custom identifiers
- **Path Skipping**: Easily exclude specific endpoints from rate limiting

## Quick Start

```go
package main

import (
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/redis/go-redis/v9"
    
    "your-project/internal/middleware"
)

func main() {
    app := fiber.New()
    
    // Initialize Redis client
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Apply rate limiter middleware
    app.Use(middleware.RateLimiter(middleware.RateLimiterConfig{
        Redis: rdb, // Can be nil to use in-memory only
        TierPolicy: map[string]middleware.RateLimitPolicy{
            "free": {
                MaxRequests:      100,
                Window:           time.Hour,
                BurstCapacity:    5,
                TokensPerSecond:  0.05, // 5 tokens per 100 seconds
                WebSocketAllowed: false,
            },
            "pro": {
                MaxRequests:      1000,
                Window:           time.Hour,
                BurstCapacity:    20,
                TokensPerSecond:  0.5, // 30 tokens per minute
                WebSocketAllowed: true,
            },
        },
        DefaultPolicy: middleware.RateLimitPolicy{
            MaxRequests:      50,
            Window:           time.Hour,
            BurstCapacity:    3,
            TokensPerSecond:  0.03, // 3 tokens per 100 seconds
            WebSocketAllowed: false,
        },
        KeyPrefix: "rl",
        GetUserID: func(c *fiber.Ctx) string {
            // Get user ID from JWT or other auth method
            return c.Locals("userId").(string)
        },
        GetUserTier: func(c *fiber.Ctx) string {
            // Get user tier from JWT or database
            return c.Locals("userTier").(string)
        },
        SkipPaths: []string{"/metrics", "/health"},
    }))
    
    // Add routes
    app.Get("/", func(c *fiber.Ctx) error {
        return c.SendString("Hello, World!")
    })
    
    app.Listen(":3000")
}
```

## Configuration Options

### RateLimiterConfig

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| Redis | *redis.Client | Redis client for storage. If nil, in-memory storage is used | nil |
| TierPolicy | map[string]RateLimitPolicy | Rate limit policies for different user tiers | {} |
| DefaultPolicy | RateLimitPolicy | Default policy applied when tier isn't found | - |
| KeyPrefix | string | Prefix for Redis keys | "rl" |
| GetUserID | func(c *fiber.Ctx) string | Function to extract user ID | - |
| GetUserTier | func(c *fiber.Ctx) string | Function to extract user tier | - |
| SkipPaths | []string | Paths to exclude from rate limiting | [] |

### RateLimitPolicy

| Option | Type | Description |
|--------|------|-------------|
| MaxRequests | int | Maximum number of requests allowed in window |
| Window | time.Duration | Time window for rate limiting |
| BurstCapacity | int | Maximum burst capacity (token bucket max tokens) |
| TokensPerSecond | float64 | Token refill rate per second |
| WebSocketAllowed | bool | Whether WebSockets are allowed for this tier |

## Storage Architecture

The middleware uses a dual-storage system:

1. **Primary Storage**: Redis for distributed rate limiting across multiple servers
2. **Fallback Storage**: In-memory map for high-availability when Redis is unavailable

The system automatically detects Redis failures and seamlessly falls back to in-memory storage.

## Token Bucket Algorithm

Unlike simple counter-based rate limiting, this middleware implements the token bucket algorithm:

1. Each user has a bucket with a maximum capacity (`BurstCapacity`)
2. Tokens are added to the bucket continuously at a rate of `TokensPerSecond`
3. Each request consumes one token
4. If the bucket has no tokens, the request is rejected
5. This allows for handling traffic bursts while maintaining overall rate limits

## WebSocket Support

WebSocket connections are handled specially:

1. WebSocket upgrade requests are detected
2. The user tier's `WebSocketAllowed` flag is checked
3. If allowed, a token is consumed from a WebSocket-specific bucket
4. If not allowed or rate limited, the connection is refused

## Response Headers

For rejected requests, the middleware returns:

- HTTP 429 Too Many Requests
- `Retry-After` header with seconds until next allowed request
- JSON response with error details and retry information

For successful requests, it sets:

- `X-RateLimit-Limit`: Maximum allowed requests
- `X-RateLimit-Remaining`: Remaining requests in the window
- `X-RateLimit-Reset`: Unix timestamp when the rate limit resets

## Advanced Configuration

### Custom User Identification

You can implement custom user identification logic:

```go
GetUserID: func(c *fiber.Ctx) string {
    // Use JWT
    token := c.Locals("user").(*jwt.Token)
    claims := token.Claims.(jwt.MapClaims)
    return claims["sub"].(string)
    
    // Or combine IP with user agent for anonymous users
    // return c.IP() + "-" + c.Get("User-Agent")
},
```

### Dynamic Tier Assignment

Implement dynamic tier assignments based on user attributes:

```go
GetUserTier: func(c *fiber.Ctx) string {
    user := c.Locals("user").(UserModel)
    
    if user.IsPremium {
        return "premium"
    } else if user.IsVerified {
        return "verified"
    }
    
    return "free"
},
```

### Integration with PostgreSQL/Other DB

If you need to check user tiers from a database:

```go
GetUserTier: func(c *fiber.Ctx) string {
    userID := c.Locals("userId").(string)
    
    // Use a cached lookup or direct DB query
    tier, err := db.QueryRow("SELECT tier FROM users WHERE id = $1", userID).Scan(&tier)
    if err != nil {
        return "free" // Default tier on error
    }
    
    return tier
},
```

## Best Practices

1. **Appropriate Tier Policies**: Set rate limits based on actual usage patterns and server capacity
2. **Token Rate Tuning**: Adjust `TokensPerSecond` to balance burst handling vs. steady traffic
3. **Redis Configuration**: Use Redis with persistence enabled to maintain rate limits across restarts
4. **Multiple Rate Limiters**: Consider using multiple rate limiters for different resources (e.g., API calls vs file uploads)
5. **Monitor and Adjust**: Regularly monitor rate limit rejections and adjust policies as needed

## Error Handling

The middleware handles errors gracefully:

- Redis connection failures trigger automatic fallback to in-memory storage
- Background jobs clean up expired entries from in-memory storage
- Proper error responses are returned with helpful information for clients



## Dependencies

```bash
go get github.com/gofiber/fiber/v2
go get github.com/gofiber/websocket/v2
go get github.com/redis/go-redis/v9
```
