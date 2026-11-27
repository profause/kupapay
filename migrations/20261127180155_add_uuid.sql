-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

ALTER TABLE auth_clients
ADD COLUMN id UUID NOT NULL DEFAULT uuid_generate_v4();

ALTER TABLE auth_clients
ADD CONSTRAINT auth_clients_pkey PRIMARY KEY (id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
