package store

import (
	"context"
	"fmt"
	"time"
)

// ConsultationRequest is a free "talk to us" lead from /consulting — not a
// paid ServiceOrder. UserID is 0 when the visitor wasn't signed in.
type ConsultationRequest struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId,omitempty"`
	Name         string    `json:"name"`
	ContactEmail string    `json:"contactEmail"`
	Message      string    `json:"message"`
	Status       string    `json:"status"` // new, contacted, closed
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Store) CreateConsultationRequest(ctx context.Context, userID int64, name, email, message string) (ConsultationRequest, error) {
	var c ConsultationRequest
	var uid *int64
	if userID != 0 {
		uid = &userID
	}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO consultation_requests (user_id, name, contact_email, message)
		VALUES ($1, $2, $3, $4)
		RETURNING id, COALESCE(user_id, 0), name, contact_email, message, status, created_at`,
		uid, name, email, message,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.ContactEmail, &c.Message, &c.Status, &c.CreatedAt)
	if err != nil {
		return ConsultationRequest{}, fmt.Errorf("store: creating consultation request: %w", err)
	}
	return c, nil
}

// ListConsultationRequests is an admin-only listing (see requireAdmin) —
// there is no per-user listing since a visitor doesn't need to sign in to
// submit one.
func (s *Store) ListConsultationRequests(ctx context.Context) ([]ConsultationRequest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(user_id, 0), name, contact_email, message, status, created_at
		FROM consultation_requests ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("store: listing consultation requests: %w", err)
	}
	defer rows.Close()

	reqs := []ConsultationRequest{}
	for rows.Next() {
		var c ConsultationRequest
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.ContactEmail, &c.Message, &c.Status, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scanning consultation request: %w", err)
		}
		reqs = append(reqs, c)
	}
	return reqs, rows.Err()
}
