package routing

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestNewSurfacesMountsCustomerAndSystemAdminPrefixesSeparately(t *testing.T) {
	app := fiber.New()
	s := NewSurfaces(Config{
		App:                   app,
		PublicGroup:           "/internal/v2",
		JWTGroup:              "/internal/v2",
		SecretKeyGroup:        "/api/v2",
		SystemAdminGroup:      "/internal/v2/admin",
		WebhookGroup:          "/webhooks",
		JWTMiddleware:         markMiddleware("jwt"),
		SecretKeyMiddleware:   markMiddleware("sk"),
		SystemAdminMiddleware: markMiddleware("system-admin"),
	})

	s.Public.Get("/auth/login", routeName("public"))
	s.JWT.Get("/users/me", routeName("jwt"))
	s.JWT.Query("/users/search", routeName("jwt-query"))
	s.SecretKey.Get("/wallets", routeName("secret-key"))
	s.SystemAdmin.Get("/settings", routeName("system-admin"))
	s.Webhooks.Post("/provider/topic", routeName("webhook"))

	tests := []struct {
		path string
		want string
	}{
		{path: "/internal/v2/auth/login", want: "public:"},
		{path: "/internal/v2/users/me", want: "jwt:jwt"},
		{path: "/api/v2/wallets", want: "secret-key:sk"},
		{path: "/internal/v2/admin/settings", want: "system-admin:jwt,system-admin"},
	}

	for _, tt := range tests {
		req, _ := http.NewRequest(http.MethodGet, tt.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET %s: %v", tt.path, err)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		_ = resp.Body.Close()

		if got := string(body); got != tt.want {
			t.Fatalf("GET %s = %q, want %q", tt.path, got, tt.want)
		}
	}

	req, _ := http.NewRequest(http.MethodGet, "/internal/v2/settings", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /internal/v2/settings: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("GET /internal/v2/settings status = %d, want %d", resp.StatusCode, fiber.StatusNotFound)
	}

	req, _ = http.NewRequest(http.MethodPost, "/webhooks/provider/topic", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("POST /webhooks/provider/topic: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read webhook response body: %v", err)
	}
	_ = resp.Body.Close()
	if got := string(body); got != "webhook:" {
		t.Fatalf("POST /webhooks/provider/topic = %q, want %q", got, "webhook:")
	}

	req, _ = http.NewRequest(fiber.MethodQuery, "/internal/v2/users/search", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("QUERY /internal/v2/users/search: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read query response body: %v", err)
	}
	_ = resp.Body.Close()
	if got := string(body); got != "jwt-query:jwt" {
		t.Fatalf("QUERY /internal/v2/users/search = %q, want %q", got, "jwt-query:jwt")
	}
}

// The service-to-service surface sits on its own /intersystem/v2 prefix,
// distinct from the JWT dashboard surface at /internal/v2 — so this pins that
// neither guard bleeds across that boundary. A shared-secret guard leaking onto
// the dashboard, or JWT onto the peer surface, would both look like working
// routes until the wrong caller got in.
func TestNewSurfacesIsolatesTheInternalServiceSurface(t *testing.T) {
	app := fiber.New()
	s := NewSurfaces(Config{
		App:                       app,
		PublicGroup:               "/internal/v2",
		JWTGroup:                  "/internal/v2",
		InternalServiceGroup:      "/intersystem/v2",
		JWTMiddleware:             markMiddleware("jwt"),
		InternalServiceMiddleware: markMiddleware("service-token"),
	})

	s.Public.Get("/auth/login", routeName("public"))
	s.JWT.Get("/users/me", routeName("jwt"))
	s.InternalService.Get("/compliance/companies/:companyId/kyc-documents", routeName("service"))

	tests := []struct {
		path string
		want string
	}{
		{path: "/intersystem/v2/compliance/companies/abc/kyc-documents", want: "service:service-token"},
		{path: "/internal/v2/users/me", want: "jwt:jwt"},
		{path: "/internal/v2/auth/login", want: "public:"},
	}

	for _, tt := range tests {
		req, _ := http.NewRequest(http.MethodGet, tt.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET %s: %v", tt.path, err)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read response body: %v", err)
		}
		_ = resp.Body.Close()

		if got := string(body); got != tt.want {
			t.Fatalf("GET %s = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func markMiddleware(name string) fiber.Handler {
	return func(c fiber.Ctx) error {
		marks, _ := c.Locals("marks").([]string)
		c.Locals("marks", append(marks, name))
		return c.Next()
	}
}

func routeName(name string) fiber.Handler {
	return func(c fiber.Ctx) error {
		marks, _ := c.Locals("marks").([]string)
		return c.SendString(name + ":" + strings.Join(marks, ","))
	}
}
