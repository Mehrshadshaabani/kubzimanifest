package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// computeSignature mirrors NOWPayments' own documented algorithm: sort all
// JSON object keys alphabetically, then HMAC-SHA512 the compact
// serialization with the IPN secret. Used here to build a known-good
// signature to test verifyNOWPaymentsSignature against.
func computeSignature(t *testing.T, payload []byte, secret string) string {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal(payload, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sorted, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(sorted)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyNOWPaymentsSignature(t *testing.T) {
	secret := "test-ipn-secret"
	// Keys deliberately out of alphabetical order, like a real IPN body.
	payload := []byte(`{"payment_status":"finished","order_id":"mflint_abc123","price_amount":"19.00"}`)
	sig := computeSignature(t, payload, secret)

	if !verifyNOWPaymentsSignature(payload, sig, secret) {
		t.Fatal("expected valid signature to verify")
	}
	if verifyNOWPaymentsSignature(payload, sig[:len(sig)-1]+"0", secret) {
		t.Fatal("expected tampered signature to fail verification")
	}
	tamperedPayload := []byte(`{"payment_status":"finished","order_id":"mflint_evil","price_amount":"19.00"}`)
	if verifyNOWPaymentsSignature(tamperedPayload, sig, secret) {
		t.Fatal("expected signature for a different payload to fail verification")
	}
	if verifyNOWPaymentsSignature(payload, sig, "wrong-secret") {
		t.Fatal("expected signature checked against the wrong secret to fail")
	}
}

func TestNOWPaymentsHandleWebhook(t *testing.T) {
	secret := "test-ipn-secret"
	p := NewNOWPaymentsProvider("test-api-key", secret, "", "")

	payload := []byte(`{"payment_status":"finished","order_id":"mflint_abc123"}`)
	sig := computeSignature(t, payload, secret)

	orderID, paid, err := p.HandleWebhook(context.Background(), payload, sig)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if orderID != "mflint_abc123" {
		t.Errorf("orderID = %q, want mflint_abc123", orderID)
	}
	if !paid {
		t.Error("expected paid=true for payment_status=finished")
	}

	pendingPayload := []byte(`{"payment_status":"waiting","order_id":"mflint_abc123"}`)
	pendingSig := computeSignature(t, pendingPayload, secret)
	_, paid, err = p.HandleWebhook(context.Background(), pendingPayload, pendingSig)
	if err != nil {
		t.Fatalf("HandleWebhook: %v", err)
	}
	if paid {
		t.Error("expected paid=false for payment_status=waiting")
	}

	if _, _, err := p.HandleWebhook(context.Background(), payload, "0000"); err == nil {
		t.Fatal("expected error for invalid signature")
	}
}
