package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/profause/kupapay/pkg/apiresponse"
	"github.com/profause/kupapay/pkg/config"
	"github.com/profause/kupapay/pkg/logger"
	"golang.org/x/crypto/bcrypt"
)

type testConfig struct {
	config *config.Config
}

func (c testConfig) Get() *config.Config { return c.config }
func (testConfig) Load() error           { return nil }

type testRepository struct {
	client *Client
}

func (r *testRepository) Create(_ context.Context, client Client) error {
	r.client = &client
	return nil
}

func (r *testRepository) ByUsername(_ context.Context, username string) (*Client, error) {
	if r.client == nil || r.client.Username != username {
		return nil, ErrNotFound
	}
	client := *r.client
	return &client, nil
}

func (r *testRepository) ByAPIKeyHash(_ context.Context, _ string) (*Client, error) {
	if r.client == nil {
		return nil, ErrNotFound
	}
	client := *r.client
	return &client, nil
}

func (r *testRepository) GetByID(_ context.Context, id uuid.UUID) (*Client, error) {
	if r.client == nil || r.client.ID != id {
		return nil, ErrNotFound
	}
	client := *r.client
	return &client, nil
}

var _ repository = (*testRepository)(nil)

func TestCreateTokenUsesConfiguredRepository(t *testing.T) {
	password := "a sufficiently long password"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := &testRepository{client: &Client{
		ID:           uuid.New(),
		Username:     "client-01",
		PasswordHash: string(passwordHash),
		Roles:        []string{"user"},
	}}
	service, err := NewService(ServiceParams{
		Config: testConfig{config: &config.Config{
			JWT: config.JWTConfig{
				Secret:    strings.Repeat("s", 32),
				ExpiresIn: time.Hour,
			},
		}},
		Store:  store,
		Logger: logger.NewNop(),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	app := fiber.New()
	app.Post("/auth/token", (&Handler{service: service}).CreateToken)
	request := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(
		`{"username":"client-01","password":"a sufficiently long password"}`,
	))
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var envelope apiresponse.SuccessEnvelope[TokenResponse]
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Status || envelope.Data.AccessToken == "" {
		t.Fatalf("unexpected token response: %+v", envelope)
	}
}
