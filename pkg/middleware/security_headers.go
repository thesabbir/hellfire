package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS protection (legacy but still useful)
		c.Header("X-XSS-Protection", "1; mode=block")

		// Control referrer information
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy
		// Restrictive policy: only allow resources from same origin
		csp := "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " + // unsafe-inline needed for some UI frameworks
			"img-src 'self' data:; " + // data: for base64 images
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		c.Header("Content-Security-Policy", csp)

		// Permissions Policy (formerly Feature-Policy)
		// Disable potentially dangerous features
		permissions := "camera=(), " +
			"microphone=(), " +
			"geolocation=(), " +
			"payment=(), " +
			"usb=(), " +
			"magnetometer=(), " +
			"gyroscope=(), " +
			"accelerometer=()"
		c.Header("Permissions-Policy", permissions)

		// HSTS (HTTP Strict Transport Security)
		// Only enable if using HTTPS (check if request is secure)
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			// max-age=31536000 (1 year), includeSubDomains
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
