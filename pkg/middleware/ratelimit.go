package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter manages rate limiters per IP address
type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit // requests per second
	b   int        // burst size
}

// NewIPRateLimiter creates a new IP-based rate limiter
// requestsPerMinute: number of requests allowed per minute
// burst: maximum burst size (allows brief spikes)
func NewIPRateLimiter(requestsPerMinute, burst int) *IPRateLimiter {
	// Convert requests per minute to requests per second for rate.Limiter
	rps := rate.Limit(float64(requestsPerMinute) / 60.0)

	limiter := &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   rps,
		b:   burst,
	}

	// Start cleanup goroutine
	go limiter.cleanupOldLimiters(5 * time.Minute)

	return limiter
}

// GetLimiter returns the rate limiter for the given IP
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// cleanupOldLimiters removes limiters for IPs that haven't been used recently
func (i *IPRateLimiter) cleanupOldLimiters(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		i.mu.Lock()
		for ip, limiter := range i.ips {
			// If limiter has full bucket (not used recently), remove it
			if limiter.Tokens() == float64(i.b) {
				delete(i.ips, ip)
			}
		}
		i.mu.Unlock()
	}
}

// RateLimitMiddleware creates a Gin middleware for rate limiting
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.GetLimiter(ip).Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitWithHeadersMiddleware creates a rate limit middleware with informative headers
func RateLimitWithHeadersMiddleware(limiter *IPRateLimiter, requestsPerMinute int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ipLimiter := limiter.GetLimiter(ip)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))

		// Get current token count (available requests)
		tokens := ipLimiter.Tokens()
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%.0f", tokens))

		if !ipLimiter.Allow() {
			// Calculate retry-after time
			reservation := ipLimiter.Reserve()
			if reservation.OK() {
				retryAfter := int(reservation.Delay().Seconds())
				c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))
				reservation.Cancel() // Don't actually consume the token
			}

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
