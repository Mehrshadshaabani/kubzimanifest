package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"mflint/internal/auth"
	"mflint/internal/store"
)

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	keys, err := s.Store.ListAPIKeys(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

type createAPIKeyRequest struct {
	Label string `json:"label"`
}

type createAPIKeyResponse struct {
	store.APIKey
	Key string `json:"key"` // shown once — the server never stores or returns this again
}

// handleCreateAPIKey mints a new API key for the caller's plan-limited use
// against /v1/lint et al. The raw key is only ever in this one response;
// only its hash is persisted (internal/auth.HashAPIKey).
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req createAPIKeyRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Label == "" {
		req.Label = "default"
	}

	raw, hash, err := auth.GenerateAPIKey()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	key, err := s.Store.CreateAPIKey(r.Context(), userID, req.Label, hash)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, createAPIKeyResponse{APIKey: key, Key: raw})
}

func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	keyID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid key id", http.StatusBadRequest)
		return
	}

	if err := s.Store.DeleteAPIKey(r.Context(), userID, keyID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "no such api key", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
