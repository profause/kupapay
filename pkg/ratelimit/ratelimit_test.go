package ratelimit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func newApp() *fiber.App {
	return fiber.New()
}

func doRequest(t *testing.T, app *fiber.App, method, path string, body string) (status int, headers map[string]string, respBody string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Second})
	if err != nil {
		t.Fatalf("Test %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	hdr := map[string]string{
		"X-RateLimit-Limit":     resp.Header.Get("X-RateLimit-Limit"),
		"X-RateLimit-Remaining": resp.Header.Get("X-RateLimit-Remaining"),
		"Retry-After":           resp.Header.Get("Retry-After"),
	}
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, hdr, string(data)
}

func TestMiddleware_AllowsUntilLimitThenBlocks(t *testing.T) {
	mw := New(nil, nil)

	app := newApp()
	app.Get("/", mw.Limit(Policy{
		Name:  "test",
		Limit: PerMinute(2),
	}), func(c fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 2; i++ {
		status, hdr, _ := doRequest(t, app, fiber.MethodGet, "/", "")
		if status != fiber.StatusOK {
			t.Fatalf("req %d: status=%d, want 200", i, status)
		}
		if hdr["X-RateLimit-Limit"] != "2" {
			t.Fatalf("req %d: X-RateLimit-Limit=%q, want 2", i, hdr["X-RateLimit-Limit"])
		}
	}

	status, hdr, body := doRequest(t, app, fiber.MethodGet, "/", "")
	if status != fiber.StatusTooManyRequests {
		t.Fatalf("3rd req status=%d, want 429", status)
	}
	if hdr["Retry-After"] == "" {
		t.Fatalf("3rd req: Retry-After missing")
	}
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if env.Message != "Too many requests" {
		t.Fatalf("message=%q, want %q", env.Message, "Too many requests")
	}
}

func TestMiddleware_KeyByJSONField(t *testing.T) {
	mw := New(nil, nil)

	app := newApp()
	app.Post("/login", mw.Limit(Policy{
		Name:  "login",
		Limit: PerMinute(1),
		Key:   KeyByJSONField("email"),
	}), func(c fiber.Ctx) error { return c.SendString("ok") })

	// First request from alice — allowed.
	status, _, _ := doRequest(t, app, fiber.MethodPost, "/login", `{"email":"alice@example.com"}`)
	if status != fiber.StatusOK {
		t.Fatalf("alice req1 status=%d, want 200", status)
	}
	// Second request from alice — blocked.
	status, _, _ = doRequest(t, app, fiber.MethodPost, "/login", `{"email":"alice@example.com"}`)
	if status != fiber.StatusTooManyRequests {
		t.Fatalf("alice req2 status=%d, want 429", status)
	}
	// Different email — independent bucket, still allowed.
	status, _, _ = doRequest(t, app, fiber.MethodPost, "/login", `{"email":"bob@example.com"}`)
	if status != fiber.StatusOK {
		t.Fatalf("bob req1 status=%d, want 200", status)
	}
}

func TestMiddleware_KeyByJSONFieldFallsBackToIPOnMalformedBody(t *testing.T) {
	mw := New(nil, nil)

	app := newApp()
	app.Post("/login", mw.Limit(Policy{
		Name:  "login-fb",
		Limit: PerMinute(1),
		Key:   KeyByJSONField("email"),
	}), func(c fiber.Ctx) error { return c.SendString("ok") })

	// Malformed JSON twice from the same IP — second is blocked because the
	// fallback IP key carries state.
	if status, _, _ := doRequest(t, app, fiber.MethodPost, "/login", `not-json`); status != fiber.StatusOK {
		t.Fatalf("req1 status=%d, want 200", status)
	}
	if status, _, _ := doRequest(t, app, fiber.MethodPost, "/login", `not-json`); status != fiber.StatusTooManyRequests {
		t.Fatalf("req2 status=%d, want 429", status)
	}
}

func TestMiddleware_FailsOpenOnLimiterError(t *testing.T) {
	mw := &Middleware{limiter: brokenLimiter{}, prefix: DefaultKeyPrefix}

	app := newApp()
	app.Get("/", mw.Limit(Policy{
		Name:  "broken",
		Limit: PerMinute(1),
	}), func(c fiber.Ctx) error { return c.SendString("ok") })

	for i := 0; i < 3; i++ {
		status, _, _ := doRequest(t, app, fiber.MethodGet, "/", "")
		if status != fiber.StatusOK {
			t.Fatalf("req %d status=%d, want 200 (fail-open)", i, status)
		}
	}
}

func TestMiddleware_NilReceiverIsPassThrough(t *testing.T) {
	var mw *Middleware

	app := newApp()
	app.Get("/", mw.Limit(Policy{Name: "noop", Limit: PerMinute(0)}), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	if status, _, _ := doRequest(t, app, fiber.MethodGet, "/", ""); status != fiber.StatusOK {
		t.Fatalf("nil middleware status=%d, want 200", status)
	}
}

func TestMiddleware_RejectsUnnamedPolicy(t *testing.T) {
	mw := New(nil, nil)

	app := newApp()
	app.Get("/", mw.Limit(Policy{Limit: PerMinute(10)}), func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	if status, _, _ := doRequest(t, app, fiber.MethodGet, "/", ""); status != fiber.StatusInternalServerError {
		t.Fatalf("unnamed policy status=%d, want 500", status)
	}
}

func TestKeyFirstNonEmpty(t *testing.T) {
	always := func(val string) KeyFunc { return func(c fiber.Ctx) string { return val } }
	composed := KeyFirstNonEmpty(always(""), nil, always("user-123"), always("never"))

	app := newApp()
	captured := ""
	app.Get("/", func(c fiber.Ctx) error {
		captured = composed(c)
		return c.SendString("ok")
	})
	_, _, _ = doRequest(t, app, fiber.MethodGet, "/", "")
	if captured != "user-123" {
		t.Fatalf("KeyFirstNonEmpty=%q, want %q", captured, "user-123")
	}
}

func TestMiddleware_CustomMessageAndOverridePrefix(t *testing.T) {
	mw := New(nil, nil).WithKeyPrefix("rl:")

	app := newApp()
	app.Get("/", mw.Limit(Policy{
		Name:    "tight",
		Limit:   PerMinute(1),
		Message: "Slow down",
	}), func(c fiber.Ctx) error { return c.SendString("ok") })

	_, _, _ = doRequest(t, app, fiber.MethodGet, "/", "")
	_, _, body := doRequest(t, app, fiber.MethodGet, "/", "")
	if !strings.Contains(body, "Slow down") {
		t.Fatalf("body=%q, want Slow down", body)
	}
}

// brokenLimiter satisfies the unexported limiter interface and forces every
// Allow call to fail so the fail-open path is exercised.
type brokenLimiter struct{}

func (brokenLimiter) Allow(context.Context, string, Limit) (*Result, error) {
	return nil, errors.New("redis down")
}
