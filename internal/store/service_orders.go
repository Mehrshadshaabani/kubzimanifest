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
	ID           int64  `json:"id"`
	UserID       int64  `json:"userId"`
	ServiceID    string `json:"serviceId"`
	PackageID    string `json:"packageId"`
	ServiceName  string `json:"serviceName"`
	PackageName  string `json:"packageName"`
	PriceUSD     int    `json:"priceUsd"`
	ContactName  string `json:"contactName"`
	ContactEmail string `json:"contactEmail"`
	// ContactPhone/ContactAddress/ContactPostalCode are collected for a
	// real card processor's billing-address requirements — optional at the
	// store layer since not every checkout needs them, required at the API
	// layer where the request actually implies a card charge (see
	// handleCreateServiceOrder).
	ContactPhone      string    `json:"contactPhone"`
	ContactAddress    string    `json:"contactAddress"`
	ContactPostalCode string    `json:"contactPostalCode"`
	ProjectNotes      string    `json:"projectNotes"`
	Status            string    `json:"status"` // pending_payment, paid, in_progress, completed, cancelled
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func (s *Store) CreateServiceOrder(ctx context.Context, o ServiceOrder) (ServiceOrder, error) {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO service_orders (user_id, service_id, package_id, service_name, package_name, price_usd, contact_name, contact_email, contact_phone, contact_address, contact_postal_code, project_notes, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'pending_payment')
		RETURNING id, user_id, service_id, package_id, service_name, package_name, price_usd, contact_name, contact_email, contact_phone, contact_address, contact_postal_code, project_notes, status, created_at, updated_at`,
		o.UserID, o.ServiceID, o.PackageID, o.ServiceName, o.PackageName, o.PriceUSD, o.ContactName, o.ContactEmail, o.ContactPhone, o.ContactAddress, o.ContactPostalCode, o.ProjectNotes,
	).Scan(&o.ID, &o.UserID, &o.ServiceID, &o.PackageID, &o.ServiceName, &o.PackageName, &o.PriceUSD, &o.ContactName, &o.ContactEmail, &o.ContactPhone, &o.ContactAddress, &o.ContactPostalCode, &o.ProjectNotes, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return ServiceOrder{}, fmt.Errorf("store: creating service order: %w", err)
	}
	return o, nil
}

func (s *Store) ListServiceOrders(ctx context.Context, userID int64) ([]ServiceOrder, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, service_id, package_id, service_name, package_name, price_usd, contact_name, contact_email, contact_phone, contact_address, contact_postal_code, project_notes, status, created_at, updated_at
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
		if err := rows.Scan(&o.ID, &o.UserID, &o.ServiceID, &o.PackageID, &o.ServiceName, &o.PackageName, &o.PriceUSD, &o.ContactName, &o.ContactEmail, &o.ContactPhone, &o.ContactAddress, &o.ContactPostalCode, &o.ProjectNotes, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scanning service order: %w", err)
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// ServiceOrderWithUser is ServiceOrder plus the owning user's email, for the
// admin orders listing (a regular user's own orders never need this since
// they already know who they are).
type ServiceOrderWithUser struct {
	ServiceOrder
	UserEmail string `json:"userEmail"`
}

func (s *Store) ListAllServiceOrders(ctx context.Context) ([]ServiceOrderWithUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, o.user_id, o.service_id, o.package_id, o.service_name, o.package_name, o.price_usd, o.contact_name, o.contact_email, o.contact_phone, o.contact_address, o.contact_postal_code, o.project_notes, o.status, o.created_at, o.updated_at, u.email
		FROM service_orders o JOIN users u ON u.id = o.user_id
		ORDER BY o.created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("store: listing all service orders: %w", err)
	}
	defer rows.Close()

	orders := []ServiceOrderWithUser{}
	for rows.Next() {
		var o ServiceOrderWithUser
		if err := rows.Scan(&o.ID, &o.UserID, &o.ServiceID, &o.PackageID, &o.ServiceName, &o.PackageName, &o.PriceUSD, &o.ContactName, &o.ContactEmail, &o.ContactPhone, &o.ContactAddress, &o.ContactPostalCode, &o.ProjectNotes, &o.Status, &o.CreatedAt, &o.UpdatedAt, &o.UserEmail); err != nil {
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
