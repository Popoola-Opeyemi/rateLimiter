package rateLimiter

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

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
