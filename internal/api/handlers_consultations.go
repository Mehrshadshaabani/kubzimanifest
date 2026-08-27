package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type consultationRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// handleCreateConsultation is public and doesn't require sign-in — a
// consultation request is a free lead, not a purchase (contrast with
// handleCreateServiceOrder). If a bearer token happens to be present it's
// used to link the request to that account, but it's optional.
func (s *Server) handleCreateConsultation(w http.ResponseWriter, r *http.Request) {
	var req consultationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	if req.Name == "" || req.Email == "" {
		http.Error(w, "name and email are required", http.StatusBadRequest)
		return
	}

	userID, _ := s.bearerUserID(r)

	created, err := s.Store.CreateConsultationRequest(r.Context(), userID, req.Name, req.Email, strings.TrimSpace(req.Message))
	if err != nil {
		log.Printf("handleCreateConsultation: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, created)
}
