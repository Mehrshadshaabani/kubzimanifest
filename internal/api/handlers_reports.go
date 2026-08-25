package api

import "net/http"

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	reports, err := s.Store.ListReports(r.Context(), userID, 50)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, reports)
}
