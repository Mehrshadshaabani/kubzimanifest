-- Free-form "book a consultation" leads from /consulting. user_id is
-- nullable: the form doesn't require sign-in, unlike a paid service_order.
CREATE TABLE IF NOT EXISTS consultation_requests (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT REFERENCES users(id) ON DELETE SET NULL,
    name          TEXT NOT NULL,
    contact_email TEXT NOT NULL,
    message       TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'new',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS consultation_requests_created_at_idx ON consultation_requests (created_at DESC);
