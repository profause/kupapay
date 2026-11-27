package auth

import (
	"github.com/profause/kupapay/internal/authctx"
	"github.com/profause/kupapay/pkg/ratelimit"
	"github.com/profause/kupapay/pkg/routing"
)

var _ routing.Mountable = (*Handler)(nil)

func (h *Handler) Mount(s *routing.Surfaces) {

	public := s.Public.Group("/auth", authctx.AttachRequestMeta)
	public.Post("/token", h.authRateLimit("auth.token", ratelimit.KeyByJSONField("username")), h.CreateToken)
	public.Post("/signup", h.authRateLimit("auth.signup", ratelimit.KeyByJSONField("username")), h.SignUp)
}
