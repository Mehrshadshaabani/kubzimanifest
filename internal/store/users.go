package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("store: not found")

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// CreateUser inserts a new user with an already-hashed password. Returns
// ErrNotFound-shaped duplicate errors as-is for the caller to translate to a
// 409/400; the caller is expected to check for a unique_violation via the
// returned error string, since this package intentionally avoids depending
// on pgx's error types here to keep the interface small.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("store: creating user: %w", err)
	}
	return u, nil
}

// GetUserByID is used by requireAdmin (internal/api/middleware.go) to check
// a signed-in user's email against the admin allowlist.
func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, created_at FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: getting user by id: %w", err)
	}
	return u, nil
}

// UpsertGoogleUser finds or creates a user by Google's stable "sub" id.
// Keyed by email on conflict, so signing in with Google using the same
// email address as an existing GitHub-linked account lands on that same
// user instead of creating a duplicate.
func (s *Store) UpsertGoogleUser(ctx context.Context, googleID, email string) (User, error) {
	return s.upsertOAuthUser(ctx, "google_id", googleID, email)
}

// UpsertGitHubUser finds or creates a user by GitHub's numeric user id
// (stringified). Same email-conflict linking behavior as UpsertGoogleUser.
func (s *Store) UpsertGitHubUser(ctx context.Context, githubID, email string) (User, error) {
	return s.upsertOAuthUser(ctx, "github_id", githubID, email)
}

// upsertOAuthUser is shared by the two methods above. column is always one
// of the two fixed string literals passed by this package, never caller
// input, so building the query with it is safe.
func (s *Store) upsertOAuthUser(ctx context.Context, column, providerID, email string) (User, error) {
	query := fmt.Sprintf(`
		INSERT INTO users (email, %s) VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET %s = EXCLUDED.%s
		RETURNING id, email, created_at`, column, column, column)

	var u User
	err := s.db.QueryRowContext(ctx, query, email, providerID).Scan(&u.ID, &u.Email, &u.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("store: upserting oauth user: %w", err)
	}
	return u, nil
}
