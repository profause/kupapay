package auth

import (
	"context"
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
		Column("id", "username", "api_key_hash", "password_hash", "roles").
		Where("username = ?", username).
		Scan(ctx)
	return model.toClient(), err
}

func (s *Store) ByAPIKeyHash(ctx context.Context, apiKeyHash string) (*Client, error) {
	var model clientModel
	err := s.db.NewSelect().
		Model(&model).
		Column("id", "username", "roles").
		Where("api_key_hash = ?", apiKeyHash).
		Scan(ctx)
	return model.toClient(), err
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (*Client, error) {
	var model clientModel
	err := s.db.NewSelect().
		Model(&model).
		Column("id", "username", "api_key_hash", "password_hash", "roles").
		Where("id = ?", id).
		Scan(ctx)
	return model.toClient(), err
}
