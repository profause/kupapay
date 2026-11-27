package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/profause/kupapay/internal/authctx"
	"github.com/profause/kupapay/pkg/apiresponse"
	"github.com/profause/kupapay/pkg/config"
	"github.com/profause/kupapay/pkg/logger"
	"github.com/profause/kupapay/pkg/ratelimit"
	"github.com/profause/kupapay/pkg/validation"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"
)

type CredentialsRequest struct {
	Username string `json:"username" example:"client-01"`
	Password string `json:"password" example:"correct horse battery staple"`
}

type SignUpResponse struct {
	Username string   `json:"username"`
	APIKey   string   `json:"api_key"`
	Roles    []string `json:"roles"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type" example:"Bearer"`
	ExpiresIn   int64  `json:"expires_in" example:"3600"`
}

// authRateWindow is the 15-minute window legacy uses on every throttled auth
// endpoint (NestJS EnhancedRateLimit({windowMs: 15 * 60 * 1000, max: 10})).
const authRateWindow = 15 * time.Minute

// authRateMax mirrors the legacy max=10 budget. Tight on purpose: these are
// credential / token / OTP endpoints where brute-force is the threat model.
const authRateMax = 10

// Handler owns the migrated auth HTTP surface.
type Handler struct {
	service     Service
	validator   *validation.Validator
	logger      *logger.Logger
	rateLimiter *ratelimit.Middleware
}

// authRateLimit returns the standard 10-per-15min handler for the given
// policy name and key strategy. Centralized so adjusting the legacy-parity
// constants happens in one place.
func (h *Handler) authRateLimit(name string, key ratelimit.KeyFunc) fiber.Handler {
	return h.rateLimiter.Limit(ratelimit.Policy{
		Name: name,
		Key:  key,
		Limit: ratelimit.Limit{
			Rate:   authRateMax,
			Burst:  authRateMax,
			Period: authRateWindow,
		},
	})
}

// SignUp godoc
// @Summary      Register a client
// @Description  Creates an account, returning a generated API key once. The API key is stored as a hash.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials body CredentialsRequest true "Client credentials"
// @Success      201 {object} apiresponse.SuccessDoc{data=SignUpResponse}
// @Failure      400 {object} apiresponse.FailureDoc
// @Failure      409 {object} apiresponse.FailureDoc
// @Failure      500 {object} apiresponse.FailureDoc
// @Router       /api/v1/auth/signup [post]
func (h *Handler) SignUp(c fiber.Ctx) error {
	var request CredentialsRequest
	if err := json.Unmarshal(c.Body(), &request); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid request body")
	}

	username := normalizeUsername(request.Username)
	if !validUsername(username) || len(request.Password) < 12 || len(request.Password) > 72 {
		return respondError(c, http.StatusBadRequest, "username or password does not meet requirements")
	}

	apiKey, err := newAPIKey()
	if err != nil {
		log.Printf("auth signup: generate API key: %v", err)
		return respondError(c, http.StatusInternalServerError, "could not create client credentials")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("auth signup: hash password: %v", err)
		return respondError(c, http.StatusInternalServerError, "could not create client credentials")
	}

	client := Client{
		Username:     username,
		APIKeyHash:   hashAPIKey(apiKey),
		PasswordHash: string(passwordHash),
		Roles:        []string{"user"},
	}
	if err := h.service.Create(c.Context(), client); err != nil {
		if errors.Is(err, ErrConflict) {
			return respondError(c, http.StatusConflict, "username is already registered")
		}
		log.Printf("auth signup: store client: %v", err)
		return respondError(c, http.StatusInternalServerError, "could not create client")
	}

	return apiresponse.Success(c, http.StatusCreated, SignUpResponse{
		Username: username,
		APIKey:   apiKey,
		Roles:    client.Roles,
	}, "Client registered")
}

// HandlerParams is the fx-resolvable parameter object for NewHandler. Add a
// dependency here once instead of editing every positional call site.
type HandlerParams struct {
	fx.In

	Service     Service
	Validator   *validation.Validator
	Logger      *logger.Logger
	RateLimiter *ratelimit.Middleware
	Config      config.Service
}

// NewHandler creates a new auth handler.
func NewHandler(p HandlerParams) *Handler {
	return &Handler{
		service:     p.Service,
		validator:   p.Validator,
		logger:      p.Logger,
		rateLimiter: p.RateLimiter,
	}
}

func keyByPrincipal(c fiber.Ctx) string {
	p, ok := authctx.From(c)
	if !ok || p == nil || p.ClientID == uuid.Nil {
		return ""
	}
	return "u:" + p.ClientID.String()
}

// CreateToken godoc
// @Summary      Create a JWT
// @Description  Exchanges a registered username and password for a signed JWT.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        credentials body CredentialsRequest true "Client credentials"
// @Success      200 {object} apiresponse.SuccessDoc{data=TokenResponse}
// @Failure      400 {object} apiresponse.FailureDoc
// @Failure      401 {object} apiresponse.FailureDoc
// @Failure      500 {object} apiresponse.FailureDoc
// @Router       /api/v1/auth/token [post]
func (h *Handler) CreateToken(c fiber.Ctx) error {
	if h == nil || h.service == nil {
		return respondError(c, http.StatusInternalServerError, "authentication unavailable")
	}

	var request CredentialsRequest
	if err := json.Unmarshal(c.Body(), &request); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid request body")
	}

	username := normalizeUsername(request.Username)
	if !validUsername(username) || request.Password == "" || len(request.Password) > 72 {
		return respondError(c, http.StatusUnauthorized, "invalid username or password")
	}
	client, err := h.service.ByUsername(c.Context(), username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return respondError(c, http.StatusUnauthorized, "invalid username or password")
		}
		log.Printf("auth token: load client: %v", err)
		return respondError(c, http.StatusInternalServerError, "could not authenticate client")
	}
	if client == nil {
		return respondError(c, http.StatusUnauthorized, "invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(client.PasswordHash), []byte(request.Password)); err != nil {
		return respondError(c, http.StatusUnauthorized, "invalid username or password")
	}

	token, expiresAt, err := h.service.IssueToken(client)
	if err != nil {
		log.Printf("auth token: issue JWT: %v", err)
		return respondError(c, http.StatusInternalServerError, "could not create access token")
	}

	return apiresponse.Success(c, http.StatusOK, TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
	}, "Access token created")
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func validUsername(username string) bool {
	if len(username) < 3 || len(username) > 64 {
		return false
	}
	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func respondError(c fiber.Ctx, status int, message string) error {
	return apiresponse.Failure(c, status, message)
}
