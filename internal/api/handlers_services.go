package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"mflint/internal/billing"
	"mflint/internal/services"
	"mflint/internal/store"
)

// handleListServices is public: the catalog is marketing content, not
// account-specific data.
func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, services.Catalog)
}

type serviceOrderRequest struct {
	ServiceID         string `json:"serviceId"`
	PackageID         string `json:"packageId"`
	ContactName       string `json:"contactName"`
	ContactEmail      string `json:"contactEmail"`
	ContactPhone      string `json:"contactPhone"`
	ContactAddress    string `json:"contactAddress"`
	ContactPostalCode string `json:"contactPostalCode"`
	ProjectNotes      string `json:"projectNotes"`
}

type serviceOrderResponse struct {
	Order       store.ServiceOrder `json:"order"`
	CheckoutURL string             `json:"checkoutUrl"`
}

// handleCreateServiceOrder records a service order and starts a checkout for
// its fixed price. Custom ("contact us") packages are rejected here — those
// are sold by talking to us directly, not through checkout.
func (s *Server) handleCreateServiceOrder(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	var req serviceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.ContactName = strings.TrimSpace(req.ContactName)
	req.ContactEmail = strings.TrimSpace(req.ContactEmail)
	req.ContactPhone = strings.TrimSpace(req.ContactPhone)
	req.ContactAddress = strings.TrimSpace(req.ContactAddress)
	req.ContactPostalCode = strings.TrimSpace(req.ContactPostalCode)
	if req.ContactName == "" || req.ContactEmail == "" || req.ContactPhone == "" || req.ContactAddress == "" || req.ContactPostalCode == "" {
		http.Error(w, "contactName, contactEmail, contactPhone, contactAddress, and contactPostalCode are required", http.StatusBadRequest)
		return
	}

	svc, pkg, ok := services.Find(req.ServiceID, req.PackageID)
	if !ok {
		http.Error(w, "unknown serviceId/packageId", http.StatusBadRequest)
		return
	}
	if pkg.Custom {
		http.Error(w, "this package is quote-based — contact us instead of ordering directly", http.StatusBadRequest)
		return
	}

	order, err := s.Store.CreateServiceOrder(r.Context(), store.ServiceOrder{
		UserID:            userID,
		ServiceID:         svc.ID,
		PackageID:         pkg.ID,
		ServiceName:       svc.Name,
		PackageName:       pkg.Name,
		PriceUSD:          pkg.PriceUSD,
		ContactName:       req.ContactName,
		ContactEmail:      req.ContactEmail,
		ContactPhone:      req.ContactPhone,
		ContactAddress:    req.ContactAddress,
		ContactPostalCode: req.ContactPostalCode,
		ProjectNotes:      strings.TrimSpace(req.ProjectNotes),
	})
	if err != nil {
		log.Printf("handleCreateServiceOrder: CreateServiceOrder: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	checkout, err := s.Billing.CreateCustomCheckout(r.Context(), float64(pkg.PriceUSD), svc.Name+" — "+pkg.Name)
	if errors.Is(err, billing.ErrNotConfigured) {
		http.Error(w, "billing is not available on this instance yet (no payment provider configured)", http.StatusNotImplemented)
		return
	}
	if err != nil {
		log.Printf("handleCreateServiceOrder: %s.CreateCustomCheckout: %v", s.Billing.Name(), err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if _, err := s.Store.CreateServiceCheckoutSession(r.Context(), userID, order.ID, s.Billing.Name(), checkout.OrderID); err != nil {
		log.Printf("handleCreateServiceOrder: CreateServiceCheckoutSession: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, serviceOrderResponse{Order: order, CheckoutURL: checkout.URL})
}

func (s *Server) handleListServiceOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	orders, err := s.Store.ListServiceOrders(r.Context(), userID)
	if err != nil {
		log.Printf("handleListServiceOrders: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}
