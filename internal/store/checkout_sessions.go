package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CheckoutSession tracks one attempt to pay for a plan, created before the
// billing provider is called so an incoming webhook (which only knows the
// provider's order id) can be resolved back to a user and plan.
type CheckoutSession struct {
	ID              int64
	UserID          int64
	Plan            string
	Provider        string
	ProviderOrderID string
	Status          string // "pending", "completed", "failed"
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s *Store) CreateCheckoutSession(ctx context.Context, userID int64, plan, provider, providerOrderID string) (CheckoutSession, error) {
	var cs CheckoutSession
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO checkout_sessions (user_id, plan, provider, provider_order_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, plan, provider, provider_order_id, status, created_at, updated_at`,
		userID, plan, provider, providerOrderID,
	).Scan(&cs.ID, &cs.UserID, &cs.Plan, &cs.Provider, &cs.ProviderOrderID, &cs.Status, &cs.CreatedAt, &cs.UpdatedAt)
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("store: creating checkout session: %w", err)
	}
	return cs, nil
}

func (s *Store) GetCheckoutSessionByOrderID(ctx context.Context, providerOrderID string) (CheckoutSession, error) {
	var cs CheckoutSession
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, plan, provider, provider_order_id, status, created_at, updated_at
		FROM checkout_sessions WHERE provider_order_id = $1`,
		providerOrderID,
	).Scan(&cs.ID, &cs.UserID, &cs.Plan, &cs.Provider, &cs.ProviderOrderID, &cs.Status, &cs.CreatedAt, &cs.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CheckoutSession{}, ErrNotFound
	}
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("store: getting checkout session: %w", err)
	}
	return cs, nil
}

func (s *Store) UpdateCheckoutSessionStatus(ctx context.Context, providerOrderID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE checkout_sessions SET status = $2, updated_at = now() WHERE provider_order_id = $1`,
		providerOrderID, status,
	)
	if err != nil {
		return fmt.Errorf("store: updating checkout session status: %w", err)
	}
	return nil
}
