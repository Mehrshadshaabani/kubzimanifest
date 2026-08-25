package api

import "net/http"

type usageResponse struct {
	Plan      string `json:"plan"`
	Limit     int    `json:"limit"` // 0 means unlimited
	Used      int    `json:"used"`
	Remaining int    `json:"remaining"` // -1 when unlimited
}

func (s *Server) handleGetUsage(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())

	sub, err := s.Store.GetSubscription(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	used, err := s.Store.GetMonthlyUsage(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	limit := planMonthlyLimit(sub.Plan)
	remaining := -1
	if limit > 0 {
		remaining = limit - used
		if remaining < 0 {
			remaining = 0
		}
	}

	writeJSON(w, http.StatusOK, usageResponse{Plan: sub.Plan, Limit: limit, Used: used, Remaining: remaining})
}
