package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thesabbir/hellfire/pkg/auth"
	"github.com/thesabbir/hellfire/pkg/logger"
)

// RequestLoggingMiddleware logs all HTTP requests with detailed information
func RequestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate unique request ID
		requestID := uuid.New().String()
		c.Set("request_id", requestID)

		// Add request ID to response headers for traceability
		c.Writer.Header().Set("X-Request-ID", requestID)

		// Record start time
		startTime := time.Now()

		// Get request details
		method := c.Request.Method
		path := c.Request.URL.Path
		queryParams := c.Request.URL.RawQuery
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// Get authenticated user if available
		username := "anonymous"
		if user := auth.GetUser(c); user != nil {
			username = user.Username
		}

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Get response status
		statusCode := c.Writer.Status()

		// Get response size
		responseSize := c.Writer.Size()

		// Build log entry
		logFields := []interface{}{
			"request_id", requestID,
			"method", method,
			"path", path,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
			"client_ip", clientIP,
			"user_agent", userAgent,
			"username", username,
			"response_size", responseSize,
		}

		if queryParams != "" {
			logFields = append(logFields, "query", queryParams)
		}

		// Log based on status code
		if statusCode >= 500 {
			logger.Error("HTTP request failed", logFields...)
		} else if statusCode >= 400 {
			logger.Warn("HTTP request client error", logFields...)
		} else {
			logger.Info("HTTP request", logFields...)
		}
	}
}
