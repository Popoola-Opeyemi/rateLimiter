# Rate Limiter

A flexible and robust rate limiting middleware for Go applications using the Fiber web framework. It supports both HTTP and WebSocket connections with configurable policies and multiple storage backends.

## Features

- Token bucket algorithm for rate limiting
- Support for both Redis and in-memory storage
- Configurable policies per user tier
- WebSocket rate limiting
- Security features:
  - Bypass tokens
  - IP whitelisting
  - Authentication-based rate limiting
  - Progressive IP blocking
  - Failed attempt tracking
- Detailed rate limit headers
- Path-based exclusions
- Automatic fallback to in-memory storage

## Installation

```bash
go get github.com/yourusername/rateLimiter
```

## Quick Start

```go
package main

import (
    "github.com/gofiber/fiber/v2"
    "github.com/yourusername/rateLimiter"
)

func main() {
    app := fiber.New()

    // Initialize rate limiter
    app.Use(rateLimiter.RateLimiter(rateLimiter.RateLimiterConfig{
        Redis: cache, // Can be nil to use in-memory only
        TierPolicy: map[string]rateLimiter.Policy{
            "free": {
                MaxRequests:      1000,
                BurstCapacity:    50,
                TokensPerSecond:  1.0,
                WebSocketAllowed: false,
                Security: rateLimiter.SecurityConfig{
                    RequireAuthentication: true,
                    MaxFailedAttempts:    5,
                    BlockDuration:        15 * time.Minute,
                },
            },
            "pro": {
                MaxRequests:      5000,
                BurstCapacity:    200,
                TokensPerSecond:  5.0,
                WebSocketAllowed: true,
                Security: rateLimiter.SecurityConfig{
                    RequireAuthentication: false,
                    MaxFailedAttempts:    10,
                    BlockDuration:        5 * time.Minute,
                },
            },
        },
        DefaultPolicy: rateLimiter.Policy{
            MaxRequests:      500,
            BurstCapacity:    25,
            TokensPerSecond:  0.5,
            WebSocketAllowed: false,
            Security: rateLimiter.SecurityConfig{
                RequireAuthentication: true,
                MaxFailedAttempts:    3,
                BlockDuration:        30 * time.Minute,
            },
        },
        GlobalSecurity: rateLimiter.SecurityConfig{
            BypassTokens: []string{
                "your-secure-bypass-token-1",
                "your-secure-bypass-token-2",
            },
            WhitelistIPs: []string{
                "127.0.0.1",
                "10.0.0.0/24",
            },
        },
        KeyPrefix: "rl",
        GetUserID: func(c *fiber.Ctx) string {
            return c.Get("X-User-ID")
        },
        GetUserTier: func(c *fiber.Ctx) string {
            return c.Get("X-User-Tier")
        },
        SkipPaths: []string{"/metrics", "/health"},
    }))

    app.Listen(":3000")
}
```

## Configuration

### Policy Configuration

Each policy defines rate limiting rules for a specific user tier:

```go
type Policy struct {
    // Maximum requests allowed in the time window
    MaxRequests int

    // Maximum number of requests allowed in a burst
    BurstCapacity int

    // Rate at which tokens are added to the bucket
    TokensPerSecond float64

    // Whether WebSocket connections are allowed
    WebSocketAllowed bool

    // Security settings for this tier
    Security SecurityConfig
}
```

### Security Configuration

Security settings can be configured globally and per tier:

```go
type SecurityConfig struct {
    // List of valid bypass tokens
    BypassTokens []string

    // List of IPs exempt from rate limiting
    WhitelistIPs []string

    // Whether to enforce stricter limits for unauthenticated requests
    RequireAuthentication bool

    // Maximum allowed failed attempts before blocking
    MaxFailedAttempts int

    // How long to block an IP after exceeding max attempts
    BlockDuration time.Duration
}
```

## Security Features

### 1. Bypass Tokens

Bypass tokens allow specific requests to skip rate limiting:

```go
// In your request
headers.Set("X-RateLimit-Bypass", "your-secure-bypass-token-1")

// In your configuration
GlobalSecurity: SecurityConfig{
    BypassTokens: []string{
        "your-secure-bypass-token-1",
        "your-secure-bypass-token-2",
    },
}
```

### 2. IP Whitelisting

Whitelist trusted IPs to bypass rate limiting:

```go
GlobalSecurity: SecurityConfig{
    WhitelistIPs: []string{
        "127.0.0.1",           // Localhost
        "10.0.0.0/24",         // Internal network
        "192.168.1.100",       // Specific IP
    },
}
```

### 3. Authentication-based Rate Limiting

Stricter rate limits for unauthenticated requests:

```go
TierPolicy: map[string]Policy{
    "free": {
        Security: SecurityConfig{
            RequireAuthentication: true,  // Stricter limits for unauthenticated requests
        },
    },
}
```

### 4. Progressive IP Blocking

IPs are blocked progressively based on failed attempts:

- Base block duration (e.g., 15 minutes)
- Each additional failed attempt adds 5 minutes
- Maximum block duration capped at 24 hours
- Automatic unblocking after duration expires

### 5. Failed Attempt Tracking

Tracks failed attempts on authentication endpoints:

```go
TierPolicy: map[string]Policy{
    "free": {
        Security: SecurityConfig{
            MaxFailedAttempts: 5,
            BlockDuration:     15 * time.Minute,
        },
    },
    "pro": {
        Security: SecurityConfig{
            MaxFailedAttempts: 10,
            BlockDuration:     5 * time.Minute,
        },
    },
}
```

## Response Headers

The rate limiter adds the following headers to responses:

- `X-RateLimit-Limit`: Maximum requests allowed
- `X-RateLimit-Remaining`: Remaining requests in the current window
- `X-RateLimit-Reset`: Time when the rate limit resets
- `Retry-After`: Seconds to wait before retrying (when rate limited)

## Error Responses

When rate limited, the middleware returns a 429 Too Many Requests response:

```json
{
    "error": "rate limit exceeded",
    "limit": 1000,
    "retry_after": 60,
    "tier": "free"
}
```

For blocked IPs:

```json
{
    "error": "IP temporarily blocked due to too many failed attempts",
    "retry_after": 900,
    "block_remaining": "15m0s",
    "failed_attempts": 5
}
```

## Storage Backends

### Redis Storage

For distributed deployments, use Redis storage:

```go
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

app.Use(rateLimiter.RateLimiter(rateLimiter.RateLimiterConfig{
    Redis: redisClient,
    // ... other config
}))
```

### In-Memory Storage

For single-instance deployments, use in-memory storage:

```go
app.Use(rateLimiter.RateLimiter(rateLimiter.RateLimiterConfig{
    Redis: nil, // Use in-memory storage
    // ... other config
}))
```

## Best Practices

1. **Configure Appropriate Limits**
   - Set reasonable burst capacities
   - Adjust token rates based on your API's capacity
   - Consider different limits for different endpoints

2. **Security Settings**
   - Use strong bypass tokens
   - Regularly rotate bypass tokens
   - Keep whitelist IPs up to date
   - Monitor failed attempt patterns

3. **Storage Considerations**
   - Use Redis for distributed deployments
   - Monitor Redis memory usage
   - Set appropriate TTLs for rate limit keys

4. **Monitoring**
   - Monitor rate limit hits and misses
   - Track blocked IPs
   - Monitor failed attempt patterns
   - Adjust limits based on usage patterns

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
