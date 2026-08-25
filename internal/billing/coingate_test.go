package billing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestCoinGateServer(t *testing.T, orderID string, status string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/orders", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token test-api-key" {
			t.Errorf("CreateCheckout: Authorization header = %q, want Token test-api-key", got)
		}
		raw, _ := io.ReadAll(r.Body)
		body, _ := url.ParseQuery(string(raw))
		if body.Get("order_id") == "" {
			t.Error("CreateCheckout: request missing order_id")
		}
		fmt.Fprintf(w, `{"id": 12345, "order_id": %q, "payment_url": "https://pay.coingate.com/checkout/12345", "status": "new"}`, body.Get("order_id"))
	})
	mux.HandleFunc("/v2/orders/12345", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Token test-api-key" {
			t.Errorf("HandleWebhook lookup: Authorization header = %q, want Token test-api-key", got)
		}
		fmt.Fprintf(w, `{"id": 12345, "order_id": %q, "status": %q}`, orderID, status)
	})
	return httptest.NewServer(mux)
}

func TestCoinGateCreateCheckout(t *testing.T) {
	srv := newTestCoinGateServer(t, "mflint_abc", "new")
	defer srv.Close()

	p := NewCoinGateProvider("test-api-key", srv.URL, "https://example.com/webhook", "https://example.com/success", "")
	checkout, err := p.CreateCheckout(context.Background(), 1, PlanTeam)
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if checkout.URL != "https://pay.coingate.com/checkout/12345" {
		t.Errorf("Checkout.URL = %q", checkout.URL)
	}
	if !strings.HasPrefix(checkout.OrderID, "mflint_") {
		t.Errorf("Checkout.OrderID = %q, want mflint_ prefix", checkout.OrderID)
	}
}

func TestCoinGateCreateCheckoutUnknownPlan(t *testing.T) {
	p := NewCoinGateProvider("test-api-key", "http://unused.invalid", "", "", "")
	if _, err := p.CreateCheckout(context.Background(), 1, PlanFree); err == nil {
		t.Error("CreateCheckout(PlanFree) should error: no price configured for the free plan")
	}
}

func TestCoinGateHandleWebhookPaid(t *testing.T) {
	srv := newTestCoinGateServer(t, "mflint_abc", "paid")
	defer srv.Close()

	p := NewCoinGateProvider("test-api-key", srv.URL, "", "", "")
	orderID, paid, err := p.HandleWebhook(context.Background(), []byte("id=12345&status=paid"), "")
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if orderID != "mflint_abc" {
		t.Errorf("orderID = %q, want mflint_abc", orderID)
	}
	if !paid {
		t.Error("paid = false, want true (order status is \"paid\" per the authenticated lookup)")
	}
}

func TestCoinGateHandleWebhookNotYetPaid(t *testing.T) {
	srv := newTestCoinGateServer(t, "mflint_abc", "confirming")
	defer srv.Close()

	p := NewCoinGateProvider("test-api-key", srv.URL, "", "", "")
	_, paid, err := p.HandleWebhook(context.Background(), []byte("id=12345&status=confirming"), "")
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if paid {
		t.Error("paid = true for a \"confirming\" order, want false — only \"paid\" should count")
	}
}

func TestCoinGateHandleWebhookIgnoresClaimedStatusInPayload(t *testing.T) {
	// The callback body itself claims "paid", but the authenticated lookup
	// (what HandleWebhook actually trusts) says the order is still "new".
	// A forged callback must not be able to fake a paid status.
	srv := newTestCoinGateServer(t, "mflint_abc", "new")
	defer srv.Close()

	p := NewCoinGateProvider("test-api-key", srv.URL, "", "", "")
	_, paid, err := p.HandleWebhook(context.Background(), []byte("id=12345&status=paid"), "")
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if paid {
		t.Error("paid = true, want false — must trust the authenticated GET /v2/orders/{id}, not the callback body's own claim")
	}
}

func TestCoinGateHandleWebhookMissingID(t *testing.T) {
	p := NewCoinGateProvider("test-api-key", "http://unused.invalid", "", "", "")
	if _, _, err := p.HandleWebhook(context.Background(), []byte("status=paid"), ""); err == nil {
		t.Error("HandleWebhook with no id should error")
	}
}
