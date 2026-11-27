package logger

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/profause/kupapay/pkg/httpx"
)

// RequestIDHeader is the HTTP header for request ID
const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware adds request ID to context and response headers.
//
// If the incoming request has an X-Request-ID header, it uses that value.
// Otherwise, it generates a new UUID.
//
// The request ID is:
//   - Added to the response header (X-Request-ID)
//   - Stored in the Fiber context locals for logging
//   - Available via c.Locals("request_id")
//
// Usage:
//
//	app := fiber.New()
//	log, _ := logger.NewService("info", "json", "stdout")
//	app.Use(logger.RequestIDMiddleware(log.Get()))
func RequestIDMiddleware(log *Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Get request ID from header or generate new one
		requestID := c.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context locals
		fiber.Locals[string](c, "request_id", requestID)

		// Add to response header
		c.Set(RequestIDHeader, requestID)

		// Create request-scoped logger and store in context
		reqLogger := log.WithRequestID(requestID)
		fiber.Locals[*Logger](c, "logger", reqLogger)

		return c.Next()
	}
}

// GetRequestID extracts the request ID from Fiber context.
//
// Returns empty string if not found.
func GetRequestID(c fiber.Ctx) string {
	return fiber.Locals[string](c, "request_id")
}

// GetLogger extracts the request-scoped logger from Fiber context.
//
// Returns the logger with request ID already attached.
// If not found, returns the provided fallback logger.
//
// Example:
//
//	func MyHandler(c fiber.Ctx, fallbackLogger *logger.Logger) error {
//	    log := logger.GetLogger(c, fallbackLogger)
//	    log.Info("Processing request") // Automatically includes request_id
//	    return c.SendString("OK")
//	}
func GetLogger(c fiber.Ctx, fallback *Logger) *Logger {
	if log := fiber.Locals[*Logger](c, "logger"); log != nil {
		return log
	}
	return fallback
}

// LoggingMiddleware logs one structured line per HTTP request on completion.
// The caller field is omitted — it would always point to this file.
//
// Requests whose path starts with any of skipPrefixes are passed through
// without logging (e.g. "/docs", "/health").
//
// Usage:
//
//	app.Use(logger.RequestIDMiddleware(log))
//	app.Use(logger.LoggingMiddleware(log, "/docs", "/health"))
func LoggingMiddleware(log *Logger, skipPrefixes ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(path, prefix) {
				return c.Next()
			}
		}

		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		status := c.Response().StatusCode()
		body := c.Request().Body()
		reqLogger := GetLogger(c, log).WithoutCaller()

		var bodyField any
		if len(body) > 0 {
			var parsed any
			if json.Unmarshal(body, &parsed) == nil {
				bodyField = parsed
			} else {
				bodyField = string(body)
			}
		}

		fields := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"body", bodyField,
			"ip", httpx.ClientIP(c),
		}

		if err != nil {
			fields = append(fields, "error", err.Error())
			reqLogger.Errorw(fmt.Sprintf("%d_request", status), fields...)
			return err
		}

		switch {
		case status >= 500:
			fields = append(fields, "response", responseBodyField(c))
			reqLogger.Errorw("5xx_request", fields...)
		case status >= 400:
			fields = append(fields, "response", responseBodyField(c))
			reqLogger.Warnw("4xx_request", fields...)
		default:
			reqLogger.Infow("request", fields...)
		}

		return nil
	}
}

// responseBodyField returns the response body as parsed JSON when possible,
// falling back to a raw string. Returns nil for an empty body.
func responseBodyField(c fiber.Ctx) any {
	body := c.Response().Body()
	if len(body) == 0 {
		return nil
	}
	var parsed any
	if json.Unmarshal(body, &parsed) == nil {
		return parsed
	}
	return string(body)
}
