package billing

import "context"

// StripeProvider documents where a card-payment provider would plug in.
// Deliberately NOT registered in cmd/server: Stripe does not support
// accounts based in Iran, so shipping this as the default would be
// misleading. NOWPaymentsProvider (nowpayments.go) is the one actually
// wired in, for crypto payments. Swap in a working card provider (Stripe
// via a supported legal entity/third party, Zarinpal for domestic IRR
// payments) by implementing Provider and registering it in cmd/server's
// wiring instead of/alongside NoopProvider.
type StripeProvider struct {
	// APIKey would hold the Stripe secret key, read from env/config.
	APIKey string
}

func NewStripeProvider(apiKey string) *StripeProvider {
	return &StripeProvider{APIKey: apiKey}
}

func (p *StripeProvider) Name() string { return "stripe" }

func (p *StripeProvider) CreateCheckout(ctx context.Context, userID int64, plan Plan) (Checkout, error) {
	// TODO: call Stripe Checkout Sessions API once a supported account exists.
	return Checkout{}, ErrNotConfigured
}

func (p *StripeProvider) HandleWebhook(ctx context.Context, payload []byte, signature string) (string, bool, error) {
	// TODO: verify webhook signature (stripe-go/webhook) and map the event
	// to (orderID, paid).
	return "", false, ErrNotConfigured
}

func (p *StripeProvider) GetSubscription(ctx context.Context, userID int64) (Subscription, error) {
	// TODO: call Stripe Subscriptions API.
	return Subscription{}, ErrNotConfigured
}
