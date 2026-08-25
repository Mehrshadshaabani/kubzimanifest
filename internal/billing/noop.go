package billing

import "context"

// NoopProvider is the default Provider: it performs no payment processing
// and reports every user as on the free plan. This is what's actually
// registered in cmd/server until a real provider is wired in.
type NoopProvider struct{}

func (NoopProvider) Name() string { return "noop" }

func (NoopProvider) CreateCheckout(ctx context.Context, userID int64, plan Plan) (Checkout, error) {
	return Checkout{}, ErrNotConfigured
}

func (NoopProvider) HandleWebhook(ctx context.Context, payload []byte, signature string) (string, bool, error) {
	return "", false, ErrNotConfigured
}

func (NoopProvider) GetSubscription(ctx context.Context, userID int64) (Subscription, error) {
	return Subscription{Plan: PlanFree, Status: "active"}, nil
}
