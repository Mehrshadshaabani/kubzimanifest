package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"mflint/internal/billing"
	"mflint/internal/store"
)

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	sub, err := s.Store.GetSubscription(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

type checkoutRequest struct {
	Plan string `json:"plan"`
}

// handleCheckout starts a real payment flow when a billing.Provider is
// configured (NOWPaymentsProvider — see cmd/server/main.go); it 501s via
// NoopProvider otherwise. The checkout session is persisted *before* the
// invoice URL is handed back, since the provider's webhook only carries its
// own order ID and needs this row to resolve which user/plan to credit.
func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req checkoutRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	plan := billing.Plan(req.Plan)
	if plan != billing.PlanTeam && plan != billing.PlanPro {
		http.Error(w, `plan must be "team" or "pro"`, http.StatusBadRequest)
		return
	}

	checkout, err := s.Billing.CreateCheckout(r.Context(), userID, plan)
	if errors.Is(err, billing.ErrNotConfigured) {
		http.Error(w, "billing is not available on this instance yet (no payment provider configured)", http.StatusNotImplemented)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := s.Store.CreateCheckoutSession(r.Context(), userID, string(plan), s.Billing.Name(), checkout.OrderID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, checkout)
}

const maxWebhookBytes = 1 << 20 // 1MB

// handleNOWPaymentsWebhook is public (registered outside the requireAuth
// group): it authenticates the request itself via the provider's HMAC
// signature instead of a bearer token.
func (s *Server) handleNOWPaymentsWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBytes))
	if err != nil {
		http.Error(w, "reading webhook body", http.StatusBadRequest)
		return
	}

	orderID, paid, err := s.Billing.HandleWebhook(r.Context(), body, r.Header.Get("x-nowpayments-sig"))
	if err != nil {
		http.Error(w, "invalid webhook", http.StatusBadRequest)
		return
	}

	session, err := s.Store.GetCheckoutSessionByOrderID(r.Context(), orderID)
	if err != nil {
		// Unknown order id: acknowledge with 200 so the provider doesn't
		// retry forever over something that will never resolve.
		w.WriteHeader(http.StatusOK)
		return
	}

	if paid {
		periodEnd := time.Now().Add(30 * 24 * time.Hour)
		sub := store.Subscription{
			UserID:           session.UserID,
			Plan:             session.Plan,
			Status:           "active",
			Provider:         session.Provider,
			ExternalID:       orderID,
			CurrentPeriodEnd: &periodEnd,
		}
		if err := s.Store.UpsertSubscription(r.Context(), sub); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		_ = s.Store.UpdateCheckoutSessionStatus(r.Context(), orderID, "completed")
	}

	w.WriteHeader(http.StatusOK)
}
