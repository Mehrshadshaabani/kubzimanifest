// Package billing defines the Provider abstraction the API talks to for
// subscription checkout and webhooks. Stripe doesn't operate in Iran, so no
// real provider is wired up in this build: NoopProvider is what's actually
// registered, and it answers every call with ErrNotConfigured so the API
// can surface a clear "billing not available yet" response instead of
// silently pretending to charge someone. StripeProvider documents the shape
// a real provider (Stripe via a third party, Zarinpal, crypto, ...) would
// implement later.
package billing

import (
	"context"
	"errors"
)

var ErrNotConfigured = errors.New("billing: no payment provider is configured yet")

type Plan string

const (
	PlanFree Plan = "free"
	PlanTeam Plan = "team"
	PlanPro  Plan = "pro"
)

// Checkout is a link (or instructions) for the user to start paying for a plan.
type Checkout struct {
	URL     string `json:"url"`
	OrderID string `json:"orderId"` // provider-assigned order/invoice id; the caller persists this to resolve the webhook later
}

// Subscription is a provider's view of a user's current plan.
type Subscription struct {
	Plan   Plan
	Status string // e.g. "active", "canceled", "past_due"
}

// Provider is what the API depends on; internal/store persists whatever a
// provider reports via UpsertSubscription, decoupling billing state from
// any one vendor's API shape.
type Provider interface {
	// Name identifies the provider for logging/telemetry, e.g. "noop", "stripe".
	Name() string
	// CreateCheckout starts a checkout flow for userID to subscribe to plan.
	CreateCheckout(ctx context.Context, userID int64, plan Plan) (Checkout, error)
	// HandleWebhook verifies a provider webhook payload and reports whether
	// the order identified by orderID (== Checkout.OrderID from
	// CreateCheckout) was paid. A payment provider doesn't know which user
	// or plan an order belongs to — the caller resolves that itself (e.g.
	// via a checkout-sessions table keyed by orderID) since that mapping
	// was created before CreateCheckout was ever called.
	HandleWebhook(ctx context.Context, payload []byte, signature string) (orderID string, paid bool, err error)
	// GetSubscription fetches the current subscription state directly from
	// the provider (used sparingly; internal/store is the source of truth
	// day-to-day).
	GetSubscription(ctx context.Context, userID int64) (Subscription, error)
}
