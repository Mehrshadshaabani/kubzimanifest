package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func yearMonth(t time.Time) string {
	return t.UTC().Format("2006-01")
}

// IncrementMonthlyUsage records one more authenticated /v1/lint call for
// userID in the current calendar month (UTC) and returns the new count.
// The upsert is atomic, so concurrent requests never lose an increment.
func (s *Store) IncrementMonthlyUsage(ctx context.Context, userID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO monthly_usage (user_id, year_month, count)
		VALUES ($1, $2, 1)
		ON CONFLICT (user_id, year_month) DO UPDATE SET count = monthly_usage.count + 1
		RETURNING count`,
		userID, yearMonth(time.Now()),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: incrementing monthly usage: %w", err)
	}
	return count, nil
}

// GetMonthlyUsage returns userID's check count for the current calendar
// month (UTC), 0 if they haven't made any authenticated calls yet.
func (s *Store) GetMonthlyUsage(ctx context.Context, userID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT count FROM monthly_usage WHERE user_id = $1 AND year_month = $2`,
		userID, yearMonth(time.Now()),
	).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("store: getting monthly usage: %w", err)
	}
	return count, nil
}
