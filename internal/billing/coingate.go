package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CoinGateProvider integrates the CoinGate crypto payment gateway
// (https://coingate.com). cmd/server registers it (in place of NoopProvider)
// when COINGATE_API_KEY is set.
//
// To test without spending real money: sign up at sandbox.coingate.com,
// grab an API key from your account's API settings, and set APIBase to
// "https://api-sandbox.coingate.com" (the default).
//
// Unlike NOWPayments (HMAC-signed callback body), CoinGate's callback isn't
// itself authenticated — the callback only tells HandleWebhook *which*
// order to look at. Authenticity comes from immediately re-fetching that
// order from CoinGate's API using our own API key (GET /v2/orders/{id}):
// an attacker can POST anything to the callback URL, but can't make that
// GET return a forged "paid" status for an order that isn't actually paid.
type CoinGateProvider struct {
	APIKey          string
	APIBase         string // e.g. https://api.coingate.com (production) or https://api-sandbox.coingate.com (sandbox)
	CallbackURL     string // public URL of POST /v1/billing/webhook/coingate
	SuccessURL      string // where CoinGate sends the user back after paying
	ReceiveCurrency string // asset settled into your CoinGate balance, e.g. "USD", "USDT", or "DO_NOT_CONVERT"
	HTTPClient      *http.Client
}

func NewCoinGateProvider(apiKey, apiBase, callbackURL, successURL, receiveCurrency string) *CoinGateProvider {
	if apiBase == "" {
		apiBase = "https://api-sandbox.coingate.com"
	}
	if receiveCurrency == "" {
		receiveCurrency = "USD"
	}
	return &CoinGateProvider{
		APIKey:          apiKey,
		APIBase:         apiBase,
		CallbackURL:     callbackURL,
		SuccessURL:      successURL,
		ReceiveCurrency: receiveCurrency,
		HTTPClient:      &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *CoinGateProvider) Name() string { return "coingate" }

type coingateOrder struct {
	ID         json.Number `json:"id"`
	Status     string      `json:"status"`
	OrderID    string      `json:"order_id"`
	PaymentURL string      `json:"payment_url"`
}

// CreateCheckout creates a CoinGate order and generates its own order ID
// (returned as Checkout.OrderID). The caller is expected to persist
// (userID, plan, OrderID) — e.g. via store.CreateCheckoutSession — *before*
// showing the payment URL to the user, since the webhook callback only
// carries CoinGate's own numeric order id, which HandleWebhook resolves
// back to this OrderID by querying CoinGate directly.
func (p *CoinGateProvider) CreateCheckout(ctx context.Context, userID int64, plan Plan) (Checkout, error) {
	price, ok := planPriceUSD[plan]
	if !ok {
		return Checkout{}, fmt.Errorf("billing: no USD price configured for plan %q", plan)
	}
	return p.createOrder(ctx, price, fmt.Sprintf("mflint %s plan", plan))
}

// CreateCustomCheckout creates a CoinGate order for an arbitrary USD amount
// — used for internal/services fixed-price packages, which aren't a
// subscription Plan.
func (p *CoinGateProvider) CreateCustomCheckout(ctx context.Context, amountUSD float64, description string) (Checkout, error) {
	return p.createOrder(ctx, amountUSD, description)
}

func (p *CoinGateProvider) createOrder(ctx context.Context, priceUSD float64, title string) (Checkout, error) {
	orderID, err := randomOrderID()
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: generating order id: %w", err)
	}

	form := url.Values{
		"order_id":         {orderID},
		"price_amount":     {fmt.Sprintf("%.2f", priceUSD)},
		"price_currency":   {"USD"},
		"receive_currency": {p.ReceiveCurrency},
		"title":            {title},
	}
	if p.CallbackURL != "" {
		form.Set("callback_url", p.CallbackURL)
	}
	if p.SuccessURL != "" {
		form.Set("success_url", p.SuccessURL)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIBase+"/v2/orders", strings.NewReader(form.Encode()))
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: building order request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Token "+p.APIKey)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: calling CoinGate: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: reading CoinGate response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return Checkout{}, fmt.Errorf("billing: CoinGate returned %d: %s", resp.StatusCode, string(respBody))
	}

	var order coingateOrder
	if err := json.Unmarshal(respBody, &order); err != nil {
		return Checkout{}, fmt.Errorf("billing: parsing CoinGate response: %w", err)
	}

	return Checkout{URL: order.PaymentURL, OrderID: orderID}, nil
}

// coingatePaidStatuses are the CoinGate order statuses that mean money
// actually landed. CoinGate briefly holds a paid order at "confirming" while
// the underlying blockchain transaction confirms; "paid" is the final state.
var coingatePaidStatuses = map[string]bool{
	"paid": true,
}

// HandleWebhook ignores signature (CoinGate's callback isn't itself signed)
// and instead treats payload as CoinGate's form-encoded callback body just
// to extract "id", then re-fetches that order from CoinGate's authenticated
// API — see the CoinGateProvider doc comment for why that's what actually
// establishes trust here.
func (p *CoinGateProvider) HandleWebhook(ctx context.Context, payload []byte, signature string) (string, bool, error) {
	form, err := url.ParseQuery(string(payload))
	if err != nil {
		return "", false, fmt.Errorf("billing: parsing CoinGate webhook payload: %w", err)
	}
	id := form.Get("id")
	if id == "" || strings.TrimSpace(id) == "" {
		return "", false, fmt.Errorf("billing: CoinGate webhook payload missing id")
	}
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return "", false, fmt.Errorf("billing: CoinGate webhook payload has non-numeric id %q", id)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.APIBase+"/v2/orders/"+id, nil)
	if err != nil {
		return "", false, fmt.Errorf("billing: building order lookup request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Token "+p.APIKey)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return "", false, fmt.Errorf("billing: fetching order from CoinGate: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("billing: reading CoinGate order lookup response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("billing: CoinGate order lookup returned %d: %s", resp.StatusCode, string(respBody))
	}

	var order coingateOrder
	if err := json.Unmarshal(respBody, &order); err != nil {
		return "", false, fmt.Errorf("billing: parsing CoinGate order lookup response: %w", err)
	}
	if order.OrderID == "" {
		return "", false, fmt.Errorf("billing: CoinGate order lookup response missing order_id")
	}

	return order.OrderID, coingatePaidStatuses[strings.ToLower(order.Status)], nil
}

// GetSubscription isn't meaningful for a one-off-order provider like
// CoinGate (it has no concept of an ongoing subscription); internal/store
// is the source of truth for plan state, populated from HandleWebhook.
func (p *CoinGateProvider) GetSubscription(ctx context.Context, userID int64) (Subscription, error) {
	return Subscription{}, ErrNotConfigured
}
