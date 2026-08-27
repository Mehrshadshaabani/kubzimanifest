package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ServiceOrder is one purchase of a fixed-price package from
// internal/services.Catalog — a consulting/engineering engagement, not a
// SaaS plan change.
type ServiceOrder struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId"`
	ServiceID    string    `json:"serviceId"`
	PackageID    string    `json:"packageId"`
	ServiceName  string    `json:"serviceName"`
	PackageName  string    `json:"packageName"`
	PriceUSD     int       `json:"priceUsd"`
	ContactName  string    `json:"contactName"`
	ContactEmail string    `json:"contactEmail"`
	ProjectNotes string    `json:"projectNotes"`
	Status       string    `json:"status"` // pending_payment, paid, in_progress, completed, cancelled
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (s *Store) CreateServiceOrder(ctx context.Context, o ServiceOrder) (ServiceOrder, error) {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO service_orders (user_id, service_id, package_id, service_name, package_name, price_usd, contact_name, contact_email, project_notes, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending_payment')
		RETURNING id, user_id, service_id, package_id, service_name, package_name, price_usd, contact_name, contact_email, project_notes, status, created_at, updated_at`,
		o.UserID, o.ServiceID, o.PackageID, o.ServiceName, o.PackageName, o.PriceUSD, o.ContactName, o.ContactEmail, o.ProjectNotes,
	).Scan(&o.ID, &o.UserID, &o.ServiceID, &o.PackageID, &o.ServiceName, &o.PackageName, &o.PriceUSD, &o.ContactName, &o.ContactEmail, &o.ProjectNotes, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return ServiceOrder{}, fmt.Errorf("store: creating service order: %w", err)
	}
	return o, nil
}

func (s *Store) ListServiceOrders(ctx context.Context, userID int64) ([]ServiceOrder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, service_id, package_id, service_name, package_name, price_usd, contact_name, contact_email, project_notes, status, created_at, updated_at
		FROM service_orders WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: listing service orders: %w", err)
	}
	defer rows.Close()

	orders := []ServiceOrder{}
	for rows.Next() {
		var o ServiceOrder
		if err := rows.Scan(&o.ID, &o.UserID, &o.ServiceID, &o.PackageID, &o.ServiceName, &o.PackageName, &o.PriceUSD, &o.ContactName, &o.ContactEmail, &o.ProjectNotes, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scanning service order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (s *Store) UpdateServiceOrderStatus(ctx context.Context, id int64, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE service_orders SET status = $2, updated_at = now() WHERE id = $1`,
		id, status,
	)
	if err != nil {
		return fmt.Errorf("store: updating service order status: %w", err)
	}
	return nil
}

// CreateServiceCheckoutSession persists (userID, serviceOrderID, providerOrderID)
// before the billing provider's invoice URL is handed back, mirroring
// CreateCheckoutSession — the webhook only knows the provider's order id.
func (s *Store) CreateServiceCheckoutSession(ctx context.Context, userID, serviceOrderID int64, provider, providerOrderID string) (ServiceCheckoutSession, error) {
	var cs ServiceCheckoutSession
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO service_checkout_sessions (user_id, service_order_id, provider, provider_order_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, service_order_id, provider, provider_order_id, status, created_at, updated_at`,
		userID, serviceOrderID, provider, providerOrderID,
	).Scan(&cs.ID, &cs.UserID, &cs.ServiceOrderID, &cs.Provider, &cs.ProviderOrderID, &cs.Status, &cs.CreatedAt, &cs.UpdatedAt)
	if err != nil {
		return ServiceCheckoutSession{}, fmt.Errorf("store: creating service checkout session: %w", err)
	}
	return cs, nil
}

type ServiceCheckoutSession struct {
	ID              int64
	UserID          int64
	ServiceOrderID  int64
	Provider        string
	ProviderOrderID string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s *Store) GetServiceCheckoutSessionByOrderID(ctx context.Context, providerOrderID string) (ServiceCheckoutSession, error) {
	var cs ServiceCheckoutSession
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, service_order_id, provider, provider_order_id, status, created_at, updated_at
		FROM service_checkout_sessions WHERE provider_order_id = $1`,
		providerOrderID,
	).Scan(&cs.ID, &cs.UserID, &cs.ServiceOrderID, &cs.Provider, &cs.ProviderOrderID, &cs.Status, &cs.CreatedAt, &cs.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServiceCheckoutSession{}, ErrNotFound
	}
	if err != nil {
		return ServiceCheckoutSession{}, fmt.Errorf("store: getting service checkout session: %w", err)
	}
	return cs, nil
}

func (s *Store) UpdateServiceCheckoutSessionStatus(ctx context.Context, providerOrderID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE service_checkout_sessions SET status = $2, updated_at = now() WHERE provider_order_id = $1`,
		providerOrderID, status,
	)
	if err != nil {
		return fmt.Errorf("store: updating service checkout session status: %w", err)
	}
	return nil
}
