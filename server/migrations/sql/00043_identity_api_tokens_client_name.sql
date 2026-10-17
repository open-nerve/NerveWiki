-- api_tokens.client_name: the name the token's MCP client last told, as shared.CleanClientName cleans it (M9
-- design 4.2, 4.5; M9/P1 design 3.16): a call that tells no name, after a restart too, is still its client's.
-- NULL: no MCP client told one. Nullable without a default, the column rewrites no row; its CHECK scans the table
-- once under the ADD COLUMN's lock, a few rows an account (M9/P1 design review A-N13).

-- +goose Up
ALTER TABLE api_tokens ADD COLUMN client_name text
    CONSTRAINT api_tokens_client_name_check CHECK (char_length(client_name) BETWEEN 1 AND 128);

-- +goose Down
ALTER TABLE api_tokens DROP COLUMN client_name;
