// Package routing exposes the route-surface abstraction used by handlers to
// mount themselves on the app. A Surface is a pre-grouped fiber.Router with
// the right middleware stack already applied — handlers reach into whichever
// surface their routes belong to instead of inventing per-scope RegisterXxx
// methods. The package is domain-free: Config takes plain fiber.Handler
// middlewares, and the cmd-side wires auth-specific ones.
package routing

import (
	"reflect"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// Surfaces holds the pre-built routers for every auth scope the app exposes.
// Add a new field when a new scope appears — every handler that needs it just
// mounts on the new field, no per-handler refactor.
type Surfaces struct {
	// Public is the unauthenticated dashboard surface (e.g. /internal/v2 with
	// no auth). Use for login, password reset, captcha endpoints.
	Public fiber.Router
	// JWT is the JWT-protected customer/dashboard API surface.
	JWT fiber.Router
	// SecretKey is the external merchant surface — secret-key auth by
	// default; X-Jwt-Auth: true falls through to JWT internally.
	SecretKey fiber.Router
	// SystemAdmin is the super-admin surface with JWT + system-admin guard,
	// pre-composed because multiple handlers mount admin-only routes on it.
	SystemAdmin fiber.Router
	// Webhooks is the unauthenticated provider-callback surface. Each inbound
	// webhook adapter authenticates its own provider request.
	Webhooks fiber.Router
	// InternalService is the service-to-service surface: trusted peers (nexus)
	// authenticated by a shared secret rather than by a user or company
	// principal. Routes here carry no authctx.Principal, so they name the
	// resource they act on in the path.
	InternalService fiber.Router
}

// Mountable is implemented by handlers that want to mount routes onto the
// app's surfaces. Each handler implements one Mount method and routes itself
// onto whichever surface fields are appropriate — no central registration
// list, no per-scope method explosion.
type Mountable interface {
	Mount(s *Surfaces)
}

// Config describes the prefixes and middlewares for each surface. cmd/api
// fills it in with the project's auth middleware; tests can fill in no-op
// handlers.
type Config struct {
	App *fiber.App

	// Group prefixes.
	PublicGroup      string // e.g. "/internal/v2"
	JWTGroup         string // e.g. "/internal/v2"
	SecretKeyGroup   string // e.g. "/api/v2"
	SystemAdminGroup string // e.g. "/internal/v2/admin"
	WebhookGroup     string // e.g. "/atlas-webhook/v2"
	// InternalServiceGroup carries a v1 in its name for a reason that is not
	// versioning: it is a contract with the caller so a peer service moves onto
	// atlas by repointing its base URL and changing no code. It is not a
	// dashboard surface, and it is not something RewriteV1ToV2 should ever
	// collapse.
	InternalServiceGroup string // e.g. "/intersystem/v2"

	// Middlewares. Nil means "no middleware for that surface" — useful for
	// tests; production wires them all.
	JWTMiddleware             fiber.Handler
	SecretKeyMiddleware       fiber.Handler
	SystemAdminMiddleware     fiber.Handler
	InternalServiceMiddleware fiber.Handler
}

// NewSurfaces builds the Surfaces. Middleware is attached to routes registered
// through each returned surface, not to the shared prefix globally. That matters
// because Public and JWT share the /internal/v2 prefix and SecretKey shares
// /api/v2 with itself — middleware must not leak across surfaces.
func NewSurfaces(cfg Config) *Surfaces {
	if cfg.App == nil {
		return &Surfaces{}
	}

	systemAdminGroup := cfg.SystemAdminGroup
	if systemAdminGroup == "" {
		systemAdminGroup = cfg.JWTGroup
	}

	return &Surfaces{
		Public:      newScopedRouter(cfg.App, cfg.PublicGroup),
		JWT:         newScopedRouter(cfg.App, cfg.JWTGroup, cfg.JWTMiddleware),
		SecretKey:   newScopedRouter(cfg.App, cfg.SecretKeyGroup, cfg.SecretKeyMiddleware),
		SystemAdmin: newScopedRouter(cfg.App, systemAdminGroup, cfg.JWTMiddleware, cfg.SystemAdminMiddleware),
		Webhooks:    newScopedRouter(cfg.App, cfg.WebhookGroup),
		InternalService: newScopedRouter(
			cfg.App, cfg.InternalServiceGroup, cfg.InternalServiceMiddleware,
		),
	}
}

type scopedRouter struct {
	base     fiber.Router
	prefix   string
	handlers []any
}

func newScopedRouter(base fiber.Router, prefix string, handlers ...any) fiber.Router {
	return &scopedRouter{base: base, prefix: cleanPath(prefix), handlers: compactHandlers(handlers)}
}

func (r *scopedRouter) Use(args ...any) fiber.Router {
	if len(args) == 0 {
		return r
	}
	if prefix, ok := args[0].(string); ok {
		return r.Group(prefix, args[1:]...)
	}
	r.handlers = append(r.handlers, compactHandlers(args)...)
	return r
}

func (r *scopedRouter) Get(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodGet}, path, handler, handlers...)
}

func (r *scopedRouter) Head(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodHead}, path, handler, handlers...)
}

func (r *scopedRouter) Post(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodPost}, path, handler, handlers...)
}

func (r *scopedRouter) Put(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodPut}, path, handler, handlers...)
}

func (r *scopedRouter) Delete(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodDelete}, path, handler, handlers...)
}

func (r *scopedRouter) Connect(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodConnect}, path, handler, handlers...)
}

func (r *scopedRouter) Options(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodOptions}, path, handler, handlers...)
}

func (r *scopedRouter) Trace(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodTrace}, path, handler, handlers...)
}

func (r *scopedRouter) Patch(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodPatch}, path, handler, handlers...)
}

func (r *scopedRouter) Query(path string, handler any, handlers ...any) fiber.Router {
	return r.Add([]string{fiber.MethodQuery}, path, handler, handlers...)
}

func (r *scopedRouter) Add(methods []string, path string, handler any, handlers ...any) fiber.Router {
	first, rest := r.routeHandlers(handler, handlers...)
	r.base.Add(methods, joinPath(r.prefix, path), first, rest...)
	return r
}

func (r *scopedRouter) All(path string, handler any, handlers ...any) fiber.Router {
	first, rest := r.routeHandlers(handler, handlers...)
	r.base.All(joinPath(r.prefix, path), first, rest...)
	return r
}

func (r *scopedRouter) Group(prefix string, handlers ...any) fiber.Router {
	combined := append([]any{}, r.handlers...)
	combined = append(combined, compactHandlers(handlers)...)
	return &scopedRouter{
		base:     r.base,
		prefix:   joinPath(r.prefix, prefix),
		handlers: combined,
	}
}

func (r *scopedRouter) Domain(host string) fiber.Router {
	return &scopedRouter{base: r.base.Domain(host), prefix: r.prefix, handlers: append([]any{}, r.handlers...)}
}

func (r *scopedRouter) RouteChain(path string) fiber.Register {
	return r.base.RouteChain(joinPath(r.prefix, path))
}

func (r *scopedRouter) Route(prefix string, fn func(router fiber.Router), name ...string) fiber.Router {
	fn(r.Group(prefix))
	if len(name) > 0 {
		r.Name(name[0])
	}
	return r
}

func (r *scopedRouter) Name(name string) fiber.Router {
	r.base.Name(name)
	return r
}

func (r *scopedRouter) routeHandlers(handler any, handlers ...any) (any, []any) {
	all := append([]any{}, r.handlers...)
	all = append(all, normalizeHandler(handler))
	all = append(all, compactHandlers(handlers)...)
	return all[0], all[1:]
}

func compactHandlers(handlers []any) []any {
	if len(handlers) == 0 {
		return nil
	}
	out := handlers[:0]
	for _, handler := range handlers {
		if !isNilHandler(handler) {
			out = append(out, normalizeHandler(handler))
		}
	}
	return out
}

func normalizeHandler(handler any) any {
	if h, ok := handler.(func(fiber.Ctx) error); ok {
		return fiber.Handler(h)
	}
	return handler
}

func isNilHandler(handler any) bool {
	if handler == nil {
		return true
	}
	value := reflect.ValueOf(handler)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Interface, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func joinPath(prefix, path string) string {
	prefix = cleanPath(prefix)
	path = cleanPath(path)
	if prefix == "" {
		if path == "" {
			return "/"
		}
		return path
	}
	if path == "" || path == "/" {
		return prefix
	}
	return strings.TrimRight(prefix, "/") + "/" + strings.TrimLeft(path, "/")
}

func cleanPath(path string) string {
	if path == "" || path == "/" {
		return path
	}
	return "/" + strings.Trim(path, "/")
}
