package main

import (
	"strings"
	"time"

	"github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/profause/kupapay/internal/auth"
	"github.com/profause/kupapay/pkg/apiresponse"
	"github.com/profause/kupapay/pkg/config"
	"github.com/profause/kupapay/pkg/logger"
	"github.com/profause/kupapay/pkg/ratelimit"
	"github.com/profause/kupapay/pkg/routing"
)

func registerRoutes(
	cfg *config.Config,
	log *logger.Logger,
	authMiddleware *auth.Middleware,
	authHandler *auth.Handler,
	rateLimiter *ratelimit.Middleware,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:     cfg.App.Name,
		ProxyHeader: fiber.HeaderXForwardedFor,
		// The body limit comes from the upload policy registry, not a magic
		// number: fiber rejects an oversized request before any handler runs, so
		// a limit below the largest policy cap would make that cap unreachable.
		// The headroom covers multipart overhead — boundaries and the other form
		// fields that ride along with the file.
		TrustProxy:         true,
		EnableIPValidation: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Loopback:  true,
			Private:   true,
			LinkLocal: true,
		},
	})

	// Global middleware.
	app.Use(recover.New())
	app.Use(logger.RequestIDMiddleware(log))
	app.Use(logger.LoggingMiddleware(log, "/docs", "/health"))
	app.Use(cors.New())

	// Global IP-keyed rate limit. Applied app-wide so /health and /docs are
	// covered too — they're cheap to serve but cheap to abuse. Tighter,
	// auth-aware policies are layered on per-endpoint inside auth.Handler.
	if globalPolicy, ok := buildGlobalRateLimitPolicy(cfg.RateLimit); ok {
		app.Use(rateLimiter.Limit(globalPolicy))
	}

	// Infrastructure routes are unversioned and stable across migration phases.
	app.Get("/health", healthCheck)
	app.Get("/docs/*", swaggo.HandlerDefault)

	// Pre-built routers per auth scope. Handlers pick which fields to mount on.
	surfaces := routing.NewSurfaces(routing.Config{
		App:           app,
		PublicGroup:   "/api/v1",
		JWTGroup:      "/api/v1",
		JWTMiddleware: authMiddleware.RequireJWT,
	})

	// Every handler that owns HTTP routes goes here.
	mountables := []routing.Mountable{
		authHandler,
	}
	for _, m := range mountables {
		m.Mount(surfaces)
	}

	return app
}

// multipartOverhead is the slack added to the body limit for multipart framing:
// part boundaries, headers, and the non-file form fields sent alongside.
const multipartOverhead = 1 << 20 // 1 MiB

// healthCheck returns the service health status.
// @Summary Check API health
// @Tags Infrastructure
// @Produce json
// @Success 200 {object} apiresponse.SuccessDoc
// @Router /health [get]
func healthCheck(c fiber.Ctx) error {
	return apiresponse.Success(c, fiber.StatusOK, fiber.Map{"status": "ok"}, "")
}

// buildGlobalRateLimitPolicy translates RateLimitConfig into a Policy. Returns
// ok=false when the limit is disabled or rate <= 0 so the app skips the
// middleware entirely rather than installing a no-op handler.
func buildGlobalRateLimitPolicy(cfg config.RateLimitConfig) (ratelimit.Policy, bool) {
	if !cfg.Enabled || cfg.GlobalRate <= 0 {
		return ratelimit.Policy{}, false
	}
	period, err := time.ParseDuration(strings.TrimSpace(cfg.GlobalPeriod))
	if err != nil || period <= 0 {
		period = time.Minute
	}
	return ratelimit.Policy{
		Name: "global",
		Limit: ratelimit.Limit{
			Rate:   cfg.GlobalRate,
			Burst:  cfg.GlobalRate,
			Period: period,
		},
		Key: ratelimit.KeyByIP(),
	}, true
}
