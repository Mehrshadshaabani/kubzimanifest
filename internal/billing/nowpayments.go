package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// NOWPaymentsProvider integrates the NOWPayments crypto payment gateway
// (https://nowpayments.io). It's the Provider cmd/server actually registers
// (in place of NoopProvider) when NOWPAYMENTS_API_KEY is set.
//
// To test without spending real money: sign up at
// account-sandbox.nowpayments.io, generate a sandbox API key + IPN secret,
// and set APIBase to "https://api-sandbox.nowpayments.io" (the default).
// The sandbox lets you simulate payment outcomes without sending funds —
// see NOWPayments' sandbox docs for the request "case" values it supports.
type NOWPaymentsProvider struct {
	APIKey      string
	IPNSecret   string
	APIBase     string // e.g. https://api.nowpayments.io (production) or https://api-sandbox.nowpayments.io (sandbox)
	CallbackURL string // public URL of POST /v1/billing/webhook/nowpayments
	HTTPClient  *http.Client
}

func NewNOWPaymentsProvider(apiKey, ipnSecret, apiBase, callbackURL string) *NOWPaymentsProvider {
	if apiBase == "" {
		apiBase = "https://api-sandbox.nowpayments.io"
	}
	return &NOWPaymentsProvider{
		APIKey:      apiKey,
		IPNSecret:   ipnSecret,
		APIBase:     apiBase,
		CallbackURL: callbackURL,
		HTTPClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *NOWPaymentsProvider) Name() string { return "nowpayments" }

// planPriceUSD mirrors the Team/Pro prices from the spec's pricing table.
// Keep in sync with billing.MonthlyCheckLimit if plans change.
var planPriceUSD = map[Plan]float64{
	PlanTeam: 19,
	PlanPro:  49,
}

type nowpaymentsInvoiceRequest struct {
	PriceAmount      float64 `json:"price_amount"`
	PriceCurrency    string  `json:"price_currency"`
	OrderID          string  `json:"order_id"`
	OrderDescription string  `json:"order_description"`
	IPNCallbackURL   string  `json:"ipn_callback_url,omitempty"`
}

type nowpaymentsInvoiceResponse struct {
	ID         string `json:"id"`
	InvoiceURL string `json:"invoice_url"`
}

// CreateCheckout creates a NOWPayments invoice and generates its own order
// ID (returned as Checkout.OrderID). The caller is expected to persist
// (userID, plan, OrderID) — e.g. via store.CreateCheckoutSession — *before*
// showing the invoice URL to the user, since HandleWebhook will only have
// the order ID to go on.
func (p *NOWPaymentsProvider) CreateCheckout(ctx context.Context, userID int64, plan Plan) (Checkout, error) {
	price, ok := planPriceUSD[plan]
	if !ok {
		return Checkout{}, fmt.Errorf("billing: no USD price configured for plan %q", plan)
	}
	return p.createInvoice(ctx, price, fmt.Sprintf("mflint %s plan", plan))
}

// CreateCustomCheckout creates a NOWPayments invoice for an arbitrary USD
// amount — used for internal/services fixed-price packages, which aren't a
// subscription Plan.
func (p *NOWPaymentsProvider) CreateCustomCheckout(ctx context.Context, amountUSD float64, description string) (Checkout, error) {
	return p.createInvoice(ctx, amountUSD, description)
}

func (p *NOWPaymentsProvider) createInvoice(ctx context.Context, priceUSD float64, description string) (Checkout, error) {
	orderID, err := randomOrderID()
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: generating order id: %w", err)
	}

	reqBody, err := json.Marshal(nowpaymentsInvoiceRequest{
		PriceAmount:      priceUSD,
		PriceCurrency:    "usd",
		OrderID:          orderID,
		OrderDescription: description,
		IPNCallbackURL:   p.CallbackURL,
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: encoding invoice request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.APIBase+"/v1/invoice", bytes.NewReader(reqBody))
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: building invoice request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: calling NOWPayments: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Checkout{}, fmt.Errorf("billing: reading NOWPayments response: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return Checkout{}, fmt.Errorf("billing: NOWPayments returned %d: %s", resp.StatusCode, string(respBody))
	}

	var invoice nowpaymentsInvoiceResponse
	if err := json.Unmarshal(respBody, &invoice); err != nil {
		return Checkout{}, fmt.Errorf("billing: parsing NOWPayments response: %w", err)
	}

	return Checkout{URL: invoice.InvoiceURL, OrderID: orderID}, nil
}

type nowpaymentsIPNPayload struct {
	OrderID       string `json:"order_id"`
	PaymentStatus string `json:"payment_status"`
}

// paidStatuses are the NOWPayments payment_status values that mean money
// actually landed. "confirmed" and "finished" both occur depending on the
// coin/network; treat either as paid.
var paidStatuses = map[string]bool{
	"confirmed": true,
	"finished":  true,
}

// HandleWebhook verifies the x-nowpayments-sig HMAC-SHA512 signature (over
// the alphabetically-key-sorted JSON body, per NOWPayments' IPN docs) and
// reports whether the order was paid.
func (p *NOWPaymentsProvider) HandleWebhook(ctx context.Context, payload []byte, signature string) (string, bool, error) {
	if !verifyNOWPaymentsSignature(payload, signature, p.IPNSecret) {
		return "", false, fmt.Errorf("billing: invalid NOWPayments webhook signature")
	}

	var ipn nowpaymentsIPNPayload
	if err := json.Unmarshal(payload, &ipn); err != nil {
		return "", false, fmt.Errorf("billing: parsing NOWPayments webhook payload: %w", err)
	}
	if ipn.OrderID == "" {
		return "", false, fmt.Errorf("billing: NOWPayments webhook payload missing order_id")
	}

	return ipn.OrderID, paidStatuses[strings.ToLower(ipn.PaymentStatus)], nil
}

// GetSubscription isn't meaningful for a one-off-invoice provider like
// NOWPayments (it has no concept of an ongoing subscription); internal/store
// is the source of truth for plan state, populated from HandleWebhook.
func (p *NOWPaymentsProvider) GetSubscription(ctx context.Context, userID int64) (Subscription, error) {
	return Subscription{}, ErrNotConfigured
}

func randomOrderID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "mflint_" + hex.EncodeToString(buf), nil
}

// verifyNOWPaymentsSignature re-serializes payload with all object keys
// sorted alphabetically (matching NOWPayments' documented IPN signing
// algorithm) and compares its HMAC-SHA512 hex digest against signature.
// json.Number preserves the original numeric formatting from the payload
// (e.g. "19.00" instead of 19) so the re-serialization matches byte-for-byte
// what NOWPayments itself hashed.
func verifyNOWPaymentsSignature(payload []byte, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}

	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return false
	}
	sorted, err := json.Marshal(v) // encoding/json always sorts map keys alphabetically
	if err != nil {
		return false
	}

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(sorted)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(strings.TrimSpace(signature))))
}
