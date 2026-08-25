package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Report struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"userId"`
	ManifestHash string          `json:"manifestHash"`
	ResultJSON   json.RawMessage `json:"result"`
	CreatedAt    time.Time       `json:"createdAt"`
}

func (s *Store) CreateReport(ctx context.Context, userID int64, manifestHash string, resultJSON []byte) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO reports (user_id, manifest_hash, result_json) VALUES ($1, $2, $3) RETURNING id`,
		userID, manifestHash, resultJSON,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: creating report: %w", err)
	}
	return id, nil
}

// ListReports returns a user's most recent reports, newest first.
func (s *Store) ListReports(ctx context.Context, userID int64, limit int) ([]Report, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, manifest_hash, result_json, created_at
		 FROM reports WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: listing reports: %w", err)
	}
	defer rows.Close()

	reports := []Report{}
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.UserID, &r.ManifestHash, &r.ResultJSON, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scanning report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}
