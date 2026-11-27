package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/profause/kupapay/pkg/database"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

var (
	ErrNotFound = errors.New("auth client not found")
	ErrConflict = errors.New("auth client already exists")
)

type Client struct {
	ID           uuid.UUID
	Username     string
	APIKeyHash   string
	PasswordHash string
	Roles        []string
}

type clientModel struct {
	bun.BaseModel `bun:"table:auth_clients,alias:auth_client"`

	ID           uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Username     string    `bun:"username,type:varchar(64),notnull,unique"`
	APIKeyHash   string    `bun:"api_key_hash,type:char(64),notnull,unique"`
	PasswordHash string    `bun:"password_hash,notnull"`
	Roles        []string  `bun:"roles,type:jsonb,notnull"`
	CreatedAt    time.Time `bun:"created_at,notnull,nullzero,default:current_timestamp"`
}

type Store struct {
	db *bun.DB
}

// NewStore creates the auth store and ensures its table exists.
func NewStore(db *database.PostgresSQL) (*Store, error) {
	if db == nil || db.DB == nil {
		return nil, errors.New("auth database is required")
	}

	store := &Store{db: db.DB}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := store.db.NewCreateTable().
		Model((*clientModel)(nil)).
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create auth_clients table: %w", err)
	}

	return store, nil
}

func (s *Store) Create(ctx context.Context, client Client) error {
	model := &clientModel{
		Username:     client.Username,
		APIKeyHash:   client.APIKeyHash,
		PasswordHash: client.PasswordHash,
		Roles:        client.Roles,
	}
	_, err := s.db.NewInsert().Model(model).Exec(ctx)
	if err != nil {
		var postgresErr pgdriver.Error
		if errors.As(err, &postgresErr) && postgresErr.Field('C') == "23505" {
			return ErrConflict
		}
		return fmt.Errorf("insert auth client: %w", err)
	}

	return nil
}

func (s *Store) ByUsername(ctx context.Context, username string) (*Client, error) {
	var model clientModel
	err := s.db.NewSelect().
		Model(&model).
		Column("username", "api_key_hash", "password_hash", "roles").
		Where("username = ?", username).
		Scan(ctx)
	return clientFromModel(model, err, "username")
}

func (s *Store) ByAPIKeyHash(ctx context.Context, apiKeyHash string) (*Client, error) {
	var model clientModel
	err := s.db.NewSelect().
		Model(&model).
		Column("username", "roles").
		Where("api_key_hash = ?", apiKeyHash).
		Scan(ctx)
	return clientFromModel(model, err, "API key")
}

func clientFromModel(model clientModel, err error, lookup string) (*Client, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return &Client{}, ErrNotFound
	}
	if err != nil {
		return &Client{}, fmt.Errorf("query auth client by %s: %w", lookup, err)
	}
	return &Client{
		ID:           model.ID,
		Username:     model.Username,
		APIKeyHash:   model.APIKeyHash,
		PasswordHash: model.PasswordHash,
		Roles:        model.Roles,
	}, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*Client, error) {
	var model clientModel
	err := s.db.NewSelect().
		Model(&model).
		Column("username", "api_key_hash", "password_hash", "roles").
		Where("id = ?", id).
		Scan(ctx)
	return clientFromModel(model, err, "id")
}
