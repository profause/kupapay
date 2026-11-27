package auth

import (
	"context"

	"github.com/google/uuid"
)

type repository interface {
	Create(context.Context, Client) error
	ByUsername(context.Context, string) (*Client, error)
	ByAPIKeyHash(context.Context, string) (*Client, error)
	GetByID(context.Context, uuid.UUID) (*Client, error)
}
