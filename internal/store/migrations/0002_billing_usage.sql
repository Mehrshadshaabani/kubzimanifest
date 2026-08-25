ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS current_period_end TIMESTAMPTZ;

-- Created before calling out to a payment provider so an incoming webhook
-- has something to resolve provider_order_id back to a user/plan.
CREATE TABLE IF NOT EXISTS checkout_sessions (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan              TEXT NOT NULL,
    provider          TEXT NOT NULL,
    provider_order_id TEXT NOT NULL UNIQUE,
    status            TEXT NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per user per calendar month; incremented atomically on each
-- authenticated /v1/lint call so plan quotas can be enforced without a
-- read-then-write race.
CREATE TABLE IF NOT EXISTS monthly_usage (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    year_month TEXT NOT NULL,
    count      INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, year_month)
);
