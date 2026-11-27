package ratelimit

import (
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/profause/kupapay/pkg/httpx"
)

// KeyByIP keys the limiter on the client IP. Fiber's httpx.ClientIP(c) already honors
// the trust-proxy config set on the app, so this is the right default in
// front of a load balancer.
func KeyByIP() KeyFunc {
	return func(c fiber.Ctx) string { return httpx.ClientIP(c) }
}

// KeyByJSONField extracts a top-level string field from the request body
// (e.g. "email" for login throttles). Returns "" when the body is missing,
// not JSON, or the field is absent/non-string — the middleware falls back to
// IP-keying in that case so a malformed request still hits a limit.
//
// Reading the body here is safe with Fiber's buffered request body: later
// c.Bind().Body(&dto) calls in the handler still see the same bytes.
func KeyByJSONField(field string) KeyFunc {
	return func(c fiber.Ctx) string {
		body := c.Request().Body()
		if len(body) == 0 {
			return ""
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return ""
		}
		raw, ok := payload[field].(string)
		if !ok {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

// KeyByHeader pulls the bucket key from a request header (e.g. "X-Api-Key").
// Empty/missing headers fall back to IP.
func KeyByHeader(header string) KeyFunc {
	return func(c fiber.Ctx) string {
		return strings.TrimSpace(c.Get(header))
	}
}

// KeyFirstNonEmpty composes key strategies, returning the first non-empty
// result. Useful for "user id if authenticated, otherwise IP" without
// reaching into authctx from this package.
func KeyFirstNonEmpty(keys ...KeyFunc) KeyFunc {
	return func(c fiber.Ctx) string {
		for _, k := range keys {
			if k == nil {
				continue
			}
			if v := strings.TrimSpace(k(c)); v != "" {
				return v
			}
		}
		return ""
	}
}
