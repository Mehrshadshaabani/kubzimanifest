package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"mflint/internal/auth"
	"mflint/internal/billing"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// bearerUserID extracts and verifies an Authorization: Bearer header, which
// is either a short-lived session JWT (issued after OAuth sign-in) or a
// long-lived API key (from the panel — see handlers_apikeys.go), told apart
// by the "mflint_" prefix. Used both by requireAuth (which rejects the
// request if this fails) and by /v1/lint's optional "save this report"
// behavior (which just treats a missing/invalid token as an anonymous
// request). s.Store may be nil only when no auth is configured at all, in
// which case there's nothing to look an API key up against.
func (s *Server) bearerUserID(r *http.Request) (int64, bool) {
	authz := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authz, "Bearer ")
	if token == "" || token == authz {
		return 0, false
	}

	if auth.IsAPIKey(token) {
		if s.Store == nil {
			return 0, false
		}
		hash := auth.HashAPIKey(token)
		userID, err := s.Store.GetUserIDByAPIKeyHash(r.Context(), hash)
		if err != nil {
			return 0, false
		}
		s.Store.TouchAPIKeyLastUsed(r.Context(), hash)
		return userID, true
	}

	id, err := s.Tokens.Verify(token)
	if err != nil {
		return 0, false
	}
	return id, true
}

// planMonthlyLimit looks up billing.MonthlyCheckLimit for a plan name as
// stored in Postgres (a plain string), defaulting to the free tier's limit
// for any unrecognized value so a bad/missing plan fails closed, not open.
func planMonthlyLimit(plan string) int {
	if limit, ok := billing.MonthlyCheckLimit[billing.Plan(plan)]; ok {
		return limit
	}
	return billing.MonthlyCheckLimit[billing.PlanFree]
}
