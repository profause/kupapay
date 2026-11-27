package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/profause/kupapay/internal/authctx"
	"github.com/profause/kupapay/pkg/config"
	"github.com/profause/kupapay/pkg/logger"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"
)

const jwtIssuer = "kupapay"

var (
	ErrClientNotFound      = errors.New("client not found")
	ErrCouldNotCreateToken = errors.New("could not create access token")
)

type Service interface {
	AuthenticateBearer(ctx context.Context, token string) (*authctx.Principal, error)
	ByUsername(ctx context.Context, username string) (*Client, error)
	GetByID(context.Context, uuid.UUID) (*Client, error)
	ByAPIKeyHash(context.Context, string) (*Client, error)
	Create(context.Context, Client) error
	IssueToken(client *Client) (string, time.Time, error)
	CreateToken(ctx context.Context, request CredentialsRequest) (*TokenResponse, error)
}

var _ Service = (*service)(nil)

type service struct {
	store     repository
	jwtSecret string
	tokenTTL  time.Duration
	logger    *logger.Logger
	now       func() time.Time
}

type ServiceParams struct {
	fx.In
	Config config.Service
	Store  repository
	Logger *logger.Logger
}

func NewService(p ServiceParams) (Service, error) {
	if p.Config == nil {
		return nil, errors.New("auth config is required")
	}
	if p.Store == nil {
		return nil, errors.New("auth store is required")
	}
	cfg := p.Config.Get()
	if cfg == nil {
		return nil, errors.New("auth config is empty")
	}
	if len(cfg.JWT.Secret) < 32 {
		return nil, errors.New("JWT_SECRET must be set to a random value of at least 32 bytes")
	}
	if cfg.JWT.ExpiresIn <= 0 {
		return nil, errors.New("JWT expiration must be positive")
	}

	return newService(p.Config, p.Store, p.Logger, time.Now), nil
}

func newService(
	cfg config.Service,
	store repository,
	log *logger.Logger,
	now func() time.Time,
) *service {
	c := cfg.Get()
	return &service{
		store:     store,
		logger:    log,
		now:       now,
		jwtSecret: c.JWT.Secret,
		tokenTTL:  c.JWT.ExpiresIn,
	}
}

func newAPIKey() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate API key: %w", err)
	}
	return "kp_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func hashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

func (s *service) AuthenticateBearer(ctx context.Context, token string) (*authctx.Principal, error) {
	return s.authenticateBearer(ctx, token, true)
}

func (s *service) IssueToken(client *Client) (string, time.Time, error) {
	if client == nil {
		return "", time.Time{}, errors.New("auth client is required")
	}

	now := time.Now().UTC()
	expiresAt := now.Add(s.tokenTTL)
	claims := tokenClaims{
		ID:    client.ID,
		Roles: client.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    jwtIssuer,
			Subject:   client.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign JWT: %w", err)
	}

	return signed, expiresAt, nil
}

func (s *service) authenticateBearer(ctx context.Context, token string, requireSecondFactor bool) (*authctx.Principal, error) {
	claims, err := parseAndVerifyHS256At(token, s.jwtSecret, s.now())
	if err != nil {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	u, err := s.store.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return &authctx.Principal{
		ClientID: u.ID,
	}, nil
}

func (s *service) ByUsername(ctx context.Context, username string) (*Client, error) {
	return s.store.ByUsername(ctx, username)
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*Client, error) {
	return s.store.GetByID(ctx, id)
}

func (s *service) ByAPIKeyHash(ctx context.Context, apiKey string) (*Client, error) {
	return s.store.ByAPIKeyHash(ctx, apiKey)
}

func (s *service) Create(ctx context.Context, client Client) error {
	return s.store.Create(ctx, client)
}

func (s *service) CreateToken(ctx context.Context, request CredentialsRequest) (*TokenResponse, error) {
	client, err := s.ByUsername(ctx, request.Username)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrClientNotFound
		}
		s.logger.Errorf("auth token: load client: %v", err)
		return nil, ErrInvalidCredentials
	}
	if client == nil {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(client.PasswordHash), []byte(request.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, expiresAt, err := s.IssueToken(client)
	if err != nil {
		log.Printf("auth token: issue JWT: %v", err)
		return nil, ErrCouldNotCreateToken
	}

	return &TokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
	}, err
}
