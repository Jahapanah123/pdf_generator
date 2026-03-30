package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/jahapanah123/pdf_generator/internal/domain"
	"golang.org/x/time/rate"
)

// In-memory per-key token bucket rate limiter
type RateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	r        rate.Limit
	burst    int
}

func NewRateLimiter(r rate.Limit, burst int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		r:        r,
		burst:    burst,
	}
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.visitors[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double check
	if limiter, exists = rl.visitors[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.r, rl.burst)
	rl.visitors[key] = limiter
	return limiter
}

func RateLimit(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if uid, exists := c.Get("user_id"); exists {
			key = uid.(string)
		}

		if !rl.getLimiter(key).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests,
				domain.NewAPIError(http.StatusTooManyRequests, "rate limit exceeded"),
			)
			return
		}

		c.Next()
	}
}
