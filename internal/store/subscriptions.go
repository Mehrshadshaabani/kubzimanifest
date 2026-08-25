package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Subscription tracks a user's plan. Until internal/billing has a real
// provider wired in, every user effectively stays on plan "free",
// provider "none".
type Subscription struct {
	UserID           int64      `json:"userId"`
	Plan             string     `json:"plan"`
	Status           string     `json:"status"`
	Provider         string     `json:"provider"`
	ExternalID       string     `json:"externalId"`
	CurrentPeriodEnd *time.Time `json:"currentPeriodEnd,omitempty"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// GetSubscription returns the user's subscription, defaulting to an unsaved
// free/active/none record if they have none yet (no row is created).
func (s *Store) GetSubscription(ctx context.Context, userID int64) (Subscription, error) {
	var sub Subscription
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, plan, status, provider, external_id, current_period_end, updated_at FROM subscriptions WHERE user_id = $1`,
		userID,
	).Scan(&sub.UserID, &sub.Plan, &sub.Status, &sub.Provider, &sub.ExternalID, &sub.CurrentPeriodEnd, &sub.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{UserID: userID, Plan: "free", Status: "active", Provider: "none"}, nil
	}
	if err != nil {
		return Subscription{}, fmt.Errorf("store: getting subscription: %w", err)
	}
	return sub, nil
}

// UpsertSubscription is used once a real billing provider (see
// internal/billing) reports a plan change via its webhook handler.
func (s *Store) UpsertSubscription(ctx context.Context, sub Subscription) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO subscriptions (user_id, plan, status, provider, external_id, current_period_end, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (user_id) DO UPDATE SET
			plan = EXCLUDED.plan,
			status = EXCLUDED.status,
			provider = EXCLUDED.provider,
			external_id = EXCLUDED.external_id,
			current_period_end = EXCLUDED.current_period_end,
			updated_at = now()`,
		sub.UserID, sub.Plan, sub.Status, sub.Provider, sub.ExternalID, sub.CurrentPeriodEnd,
	)
	if err != nil {
		return fmt.Errorf("store: upserting subscription: %w", err)
	}
	return nil
}
