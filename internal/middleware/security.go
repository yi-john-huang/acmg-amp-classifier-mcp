package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SecurityHeaders adds security headers to all responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")

		// Enforce HTTPS (only in production)
		if gin.Mode() == gin.ReleaseMode {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Content Security Policy for medical applications
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")

		// Referrer policy for privacy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// CorrelationID adds a unique correlation ID to each request for audit trails
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if correlation ID already exists in headers
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Set correlation ID in context and response header
		c.Set("correlation_id", correlationID)
		c.Header("X-Correlation-ID", correlationID)

		c.Next()
	}
}

// RequestTimeout sets a timeout for all requests to prevent resource exhaustion
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return gin.TimeoutWithHandler(timeout, func(c *gin.Context) {
		c.JSON(408, gin.H{
			"error":          "Request timeout",
			"correlation_id": c.GetString("correlation_id"),
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		})
	})
}

// AuditLogger logs security-relevant events for medical compliance
func AuditLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Custom log format for medical audit requirements
		return fmt.Sprintf(`{"timestamp":"%s","correlation_id":"%s","method":"%s","path":"%s","status":%d,"latency":"%s","client_ip":"%s","user_agent":"%s","response_size":%d}%s`,
			param.TimeStamp.Format(time.RFC3339),
			param.Keys["correlation_id"],
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Request.UserAgent(),
			param.BodySize,
			"\n",
		)
	})
}
