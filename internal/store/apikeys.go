package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// APIKey is what a user sees in their panel — never the raw key (that's
// shown exactly once, at creation time, by CreateAPIKey's caller).
type APIKey struct {
	ID         int64      `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

// CreateAPIKey records a new key by its hash (see internal/auth.HashAPIKey)
// — the raw key itself is never stored.
func (s *Store) CreateAPIKey(ctx context.Context, userID int64, label, keyHash string) (APIKey, error) {
	var k APIKey
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id, key_hash, label) VALUES ($1, $2, $3) RETURNING id, label, created_at`,
		userID, keyHash, label,
	).Scan(&k.ID, &k.Label, &k.CreatedAt)
	if err != nil {
		return APIKey{}, fmt.Errorf("store: creating api key: %w", err)
	}
	return k, nil
}

// ListAPIKeys returns a user's keys, newest first.
func (s *Store) ListAPIKeys(ctx context.Context, userID int64) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, label, created_at, last_used_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: listing api keys: %w", err)
	}
	defer rows.Close()

	keys := []APIKey{}
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Label, &k.CreatedAt, &k.LastUsedAt); err != nil {
			return nil, fmt.Errorf("store: scanning api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// DeleteAPIKey revokes a key, scoped to userID so one user can't delete
// another's key by guessing an id.
func (s *Store) DeleteAPIKey(ctx context.Context, userID, keyID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`, keyID, userID)
	if err != nil {
		return fmt.Errorf("store: deleting api key: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetUserIDByAPIKeyHash resolves an API key (by its hash) to the user it
// belongs to, for request authentication.
func (s *Store) GetUserIDByAPIKeyHash(ctx context.Context, keyHash string) (int64, error) {
	var userID int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM api_keys WHERE key_hash = $1`, keyHash).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("store: looking up api key: %w", err)
	}
	return userID, nil
}

// TouchAPIKeyLastUsed best-effort records that a key was just used.
func (s *Store) TouchAPIKeyLastUsed(ctx context.Context, keyHash string) {
	_, _ = s.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = now() WHERE key_hash = $1`, keyHash)
}
