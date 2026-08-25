-- Auth is OAuth-only (Google + GitHub) — no password to store, and an
-- account is keyed by email so signing in with either provider on the same
-- email address lands on the same user.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id TEXT UNIQUE;
