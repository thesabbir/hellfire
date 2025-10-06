package errors

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/thesabbir/hellfire/pkg/logger"
)

// Common error messages (generic to avoid information leakage)
const (
	ErrAuthentication  = "authentication failed"
	ErrUnauthorized    = "unauthorized access"
	ErrForbidden       = "insufficient permissions"
	ErrNotFound        = "resource not found"
	ErrBadRequest      = "invalid request"
	ErrInternalServer  = "internal server error"
	ErrValidation      = "validation failed"
	ErrRateLimit       = "rate limit exceeded"
	ErrInvalidCSRF     = "invalid CSRF token"
	ErrInvalidInput    = "invalid input"
	ErrOperationFailed = "operation failed"
)

// RespondWithError sends a generic error response and logs the detailed error
func RespondWithError(c *gin.Context, statusCode int, genericMessage string, detailedError error) {
	// Log the detailed error for debugging (not sent to client)
	if detailedError != nil {
		logger.Error("Request error",
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"status", statusCode,
			"error", detailedError.Error(),
			"client_ip", c.ClientIP())
	}

	// Send generic error to client
	c.JSON(statusCode, gin.H{
		"error": genericMessage,
	})
}

// Convenience functions for common error scenarios

func BadRequest(c *gin.Context, err error) {
	RespondWithError(c, http.StatusBadRequest, ErrBadRequest, err)
}

func Unauthorized(c *gin.Context, err error) {
	RespondWithError(c, http.StatusUnauthorized, ErrAuthentication, err)
}

func Forbidden(c *gin.Context, err error) {
	RespondWithError(c, http.StatusForbidden, ErrForbidden, err)
}

func NotFound(c *gin.Context, err error) {
	RespondWithError(c, http.StatusNotFound, ErrNotFound, err)
}

func InternalServerError(c *gin.Context, err error) {
	RespondWithError(c, http.StatusInternalServerError, ErrInternalServer, err)
}

func ValidationError(c *gin.Context, err error) {
	RespondWithError(c, http.StatusBadRequest, ErrValidation, err)
}

func OperationFailed(c *gin.Context, err error) {
	RespondWithError(c, http.StatusInternalServerError, ErrOperationFailed, err)
}
