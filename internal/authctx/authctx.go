// Package authctx is the shared request-principal kernel. It exists so that
// domain packages (user, activitylog, …) and the auth middleware can agree on
// the authenticated principal type without importing each other (the legacy
// @AuthUser() decorator / AuthUserInterceptor is the equivalent seam).
//
// This is a complete, dependency-free package — not a deferral indirection.
package authctx

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/profause/kupapay/pkg/httpx"
)

// localsKey is the fiber Locals key the auth middleware writes the principal to.
const localsKey = "principal"

// Principal is the authenticated caller. It carries exactly what handlers and
// domain services read off the legacy hydrated UserEntity. SK auth resolves a
// company principal (UserID zero); JWT resolves a user principal.
type Principal struct {
	ClientID uuid.UUID
}

// With stores the principal on the request (called by the auth middleware).
func With(c fiber.Ctx, p *Principal) {
	c.Locals(localsKey, p)
}

// From returns the authenticated principal, or false if the request is
// unauthenticated.
func From(c fiber.Ctx) (*Principal, bool) {
	p, ok := c.Locals(localsKey).(*Principal)
	return p, ok && p != nil
}

// RequestMeta carries per-request side-effect metadata (IP, device, HTTP
// method). Domain services pull it from context.Context rather than taking
// it as a parameter on every method.
type RequestMeta struct {
	IPAddress string
	Device    string
	Method    string
	AppID     string // client|admin
}

// PasskeyAppHeader is the request header whose value ("admin"/"client") selects
// the WebAuthn relying-party profile. Set by the gateway per frontend.
const PasskeyAppHeader = "X-Brails-App"

type requestMetaKey struct{}

// WithRequestMeta returns a copy of ctx with the meta attached.
func WithRequestMeta(ctx context.Context, m RequestMeta) context.Context {
	return context.WithValue(ctx, requestMetaKey{}, m)
}

// RequestMetaFrom returns the attached meta or a zero value.
func RequestMetaFrom(ctx context.Context) RequestMeta {
	if m, ok := ctx.Value(requestMetaKey{}).(RequestMeta); ok {
		return m
	}
	return RequestMeta{}
}

// AttachRequestMeta is a fiber middleware that derives RequestMeta from the
// request and re-binds the user context. Mount it once on each route group
// that has handlers needing meta.
func AttachRequestMeta(c fiber.Ctx) error {
	meta := RequestMeta{IPAddress: httpx.ClientIP(c), Device: c.Get("User-Agent"), Method: c.Method(), AppID: c.Get(PasskeyAppHeader)}
	c.SetContext(WithRequestMeta(c.Context(), meta))
	return c.Next()
}
