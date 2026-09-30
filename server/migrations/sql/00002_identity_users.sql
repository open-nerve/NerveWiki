-- users: an account and its profile (M1 design 4). The use cases write every
-- column, the times from their clock; there are no DEFAULT now() fallbacks
-- (v0.1 design 7.1).

-- +goose Up
CREATE TABLE users (
    id uuid PRIMARY KEY,
    -- The domain lower-cases and trims the address before it validates it;
    -- the CHECK stops writes that bypass it.
    email varchar(255) NOT NULL CONSTRAINT users_email_key UNIQUE
        CONSTRAINT users_email_check CHECK (email = lower(email) AND email !~ '[[:space:]]'),
    -- The argon2id hash in PHC form.
    password varchar(128) NOT NULL,
    display_name varchar(100) NOT NULL CONSTRAINT users_display_name_check CHECK (display_name <> ''),
    is_active boolean NOT NULL DEFAULT true,
    -- The ids of the onboarding steps the account has completed; the web app's
    -- registry defines the steps (M1 design 8).
    onboarding_steps text[] NOT NULL DEFAULT '{}'
        -- Joined with commas, the ids must read as a list of ids: an empty
        -- element would leave a stray comma, or nothing at all for {""}, which
        -- the cardinality tells apart from the empty array. array_to_string
        -- skips NULLs, so they are checked apart.
        CONSTRAINT users_onboarding_steps_check CHECK (
            cardinality(onboarding_steps) <= 32
            AND array_position(onboarding_steps, NULL) IS NULL
            AND (cardinality(onboarding_steps) = 0
                OR array_to_string(onboarding_steps, ',') ~ '^[a-z][a-z0-9_]{0,31}(,[a-z][a-z0-9_]{0,31})*$')
        ),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE users;
