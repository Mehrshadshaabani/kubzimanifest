package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// handleAdminListServiceOrders is registered behind requireAuth+requireAdmin
// (see server.go) — every user's service orders, not just the caller's own.
func (s *Server) handleAdminListServiceOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.Store.ListAllServiceOrders(r.Context())
	if err != nil {
		log.Printf("handleAdminListServiceOrders: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

var validServiceOrderStatuses = map[string]bool{
	"pending_payment": true,
	"paid":            true,
	"in_progress":     true,
	"completed":       true,
	"cancelled":       true,
}

type adminUpdateOrderStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleAdminUpdateServiceOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	var req adminUpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !validServiceOrderStatuses[req.Status] {
		http.Error(w, `status must be one of pending_payment, paid, in_progress, completed, cancelled`, http.StatusBadRequest)
		return
	}

	if err := s.Store.UpdateServiceOrderStatus(r.Context(), id, req.Status); err != nil {
		log.Printf("handleAdminUpdateServiceOrderStatus: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleAdminListConsultations(w http.ResponseWriter, r *http.Request) {
	reqs, err := s.Store.ListConsultationRequests(r.Context())
	if err != nil {
		log.Printf("handleAdminListConsultations: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, reqs)
}

// handleAdminWhoAmI lets the admin.html frontend distinguish "not signed
// in" from "signed in but not an admin" from "signed in as admin" with one
// cheap call, instead of inferring it from a 403 on the data endpoints.
func (s *Server) handleAdminWhoAmI(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"admin": true})
}
