package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/profause/kupapay/internal/authctx"
	"github.com/profause/kupapay/pkg/apiresponse"
)

type principalKey struct{}

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrAmbiguousCredentials = errors.New("provide either an API key or a JWT, not both")

func (m *Middleware) RequireRoles(roles ...string) fiber.Handler {
	if len(roles) == 0 {
		panic("at least one role is required")
	}

	return func(c fiber.Ctx) error {
		principal, ok := PrincipalFrom(c)
		if !ok {
			c.Set("WWW-Authenticate", "Bearer")
			return apiresponse.Failure(c, http.StatusUnauthorized, "authentication required")
		}

		for _, principalRole := range principal.Roles {
			for _, requiredRole := range roles {
				if principalRole == requiredRole {
					return c.Next()
				}
			}
		}

		return apiresponse.Failure(c, http.StatusForbidden, "insufficient permissions")
	}
}

func bearerToken(authHeader string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", false
	}
	tok := strings.TrimPrefix(authHeader, prefix)
	return tok, tok != ""
}

func PrincipalFrom(c fiber.Ctx) (Principal, bool) {
	principal, ok := c.Locals(principalKey{}).(Principal)
	return principal, ok
}

func isPublicAuthRoute(c fiber.Ctx) bool {
	if c.Method() == fiber.MethodOptions {
		return true
	}
	if c.Method() != fiber.MethodPost {
		return false
	}

	switch c.Path() {
	case "/api/v1/auth/signup", "/api/v1/auth/token":
		return true
	default:
		return false
	}
}

type Middleware struct {
	service Service
}

func NewMiddleware(service Service) *Middleware {
	return &Middleware{service: service}
}

// RequireJWT is the fiber middleware handler. Mount it on the
// `/api/v1` (JWT) surface group in cmd/api/routes.go.
func (m *Middleware) RequireJWT(c fiber.Ctx) error {
	token, ok := bearerToken(c.Get("Authorization"))
	if !ok || m.service == nil {
		c.Set("WWW-Authenticate", "Bearer")
		return apiresponse.Failure(c, fiber.StatusUnauthorized, "authentication required")
	}

	principal, err := m.service.AuthenticateBearer(c.Context(), token)
	if err != nil {
		c.Set("WWW-Authenticate", "Bearer")
		return apiresponse.Failure(c, fiber.StatusUnauthorized, "authentication required")
	}

	authctx.With(c, principal)
	return c.Next()
}

func (m *Middleware) RequireApiKey(c fiber.Ctx) error {
	apiKey := c.Get("X-API-Key")
	if m.service == nil {
		log.Printf("auth API key middleware: service is not configured")
		return apiresponse.Failure(c, fiber.StatusInternalServerError, "authentication unavailable")
	}
	if apiKey == "" || len(apiKey) > 128 {
		return apiresponse.Failure(c, fiber.StatusUnauthorized, "authentication required")
	}

	client, err := m.service.ByAPIKeyHash(c.Context(), hashAPIKey(apiKey))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return apiresponse.Failure(c, fiber.StatusUnauthorized, "authentication required")
		}
		log.Printf("auth API key middleware: lookup client: %v", err)
		return apiresponse.Failure(c, fiber.StatusInternalServerError, "authentication unavailable")
	}
	if client == nil {
		return apiresponse.Failure(c, fiber.StatusUnauthorized, "authentication required")
	}

	authctx.With(c, &authctx.Principal{ClientID: client.ID})
	return c.Next()
}
