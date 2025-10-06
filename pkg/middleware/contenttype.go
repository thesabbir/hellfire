package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ContentTypeValidationMiddleware validates that requests with bodies have appropriate Content-Type headers
func ContentTypeValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only validate for methods that typically have request bodies
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "PATCH" {
			c.Next()
			return
		}

		// Check if request has a body
		if c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		// Get Content-Type header
		contentType := c.GetHeader("Content-Type")
		if contentType == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Content-Type header is required for requests with body",
			})
			c.Abort()
			return
		}

		// Validate Content-Type (support application/json and multipart/form-data)
		// Extract the media type (ignore charset and other parameters)
		mediaType := strings.Split(contentType, ";")[0]
		mediaType = strings.TrimSpace(mediaType)

		validTypes := map[string]bool{
			"application/json":                  true,
			"application/x-www-form-urlencoded": true,
			"multipart/form-data":               true,
		}

		if !validTypes[mediaType] {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{
				"error": "Unsupported Content-Type. Expected application/json",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
