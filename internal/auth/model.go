package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Client struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	APIKeyHash   string    `json:"apiKey"`
	PasswordHash string    `json:"password" validate:"omitempty"`
	Roles        []string  `json:"roles"`
}

type clientModel struct {
	bun.BaseModel `bun:"table:auth_clients,alias:auth_client"`
	ID            uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Username      string    `bun:"username,type:varchar(64),notnull,unique"`
	APIKeyHash    string    `bun:"api_key_hash,type:char(64),notnull,unique"`
	PasswordHash  string    `bun:"password_hash,notnull"`
	Roles         []string  `bun:"roles,type:jsonb,notnull"`
	CreatedAt     time.Time `bun:"created_at,notnull,nullzero,default:current_timestamp"`
}

func (model *clientModel) toClient() *Client {
	return &Client{
		ID:           model.ID,
		Username:     model.Username,
		APIKeyHash:   model.APIKeyHash,
		PasswordHash: model.PasswordHash,
		Roles:        model.Roles,
	}
}

type tokenClaims struct {
	ID    uuid.UUID `json:"id"`
	Roles []string  `json:"roles"`
	jwt.RegisteredClaims
}

type Principal struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Roles    []string  `json:"roles"`
}
