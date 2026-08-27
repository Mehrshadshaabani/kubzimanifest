-- Fixed-price consulting/service orders (see internal/services for the
-- catalog these reference by service_id/package_id). A separate table from
-- checkout_sessions (which is subscription-plan-specific) so a service
-- purchase never touches a user's SaaS plan.
CREATE TABLE IF NOT EXISTS service_orders (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_id    TEXT NOT NULL,
    package_id    TEXT NOT NULL,
    service_name  TEXT NOT NULL,
    package_name  TEXT NOT NULL,
    price_usd     INT NOT NULL,
    contact_name  TEXT NOT NULL DEFAULT '',
    contact_email TEXT NOT NULL DEFAULT '',
    project_notes TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending_payment',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS service_orders_user_id_idx ON service_orders (user_id, created_at DESC);

-- Created before calling out to the billing provider, mirroring
-- checkout_sessions, so an incoming webhook can resolve provider_order_id
-- back to the service_order it's paying for.
CREATE TABLE IF NOT EXISTS service_checkout_sessions (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    service_order_id  BIGINT NOT NULL REFERENCES service_orders(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_order_id TEXT NOT NULL UNIQUE,
    status            TEXT NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
