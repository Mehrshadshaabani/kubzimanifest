package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"mflint/internal/cost"
	"mflint/internal/parser"
	"mflint/internal/report"
	"mflint/internal/rules"
)

type lintJSONRequest struct {
	Manifest string `json:"manifest"`
	Cloud    string `json:"cloud"`
	Save     bool   `json:"save"`
	NoCost   bool   `json:"noCost"`
}

const maxManifestBytes = 2 << 20 // 2MB

// handleLint is public and unauthenticated: paste-YAML-get-a-report is the
// free, no-login web/API path described in the spec. If the caller both
// sends a valid bearer token AND asks to save (JSON body "save": true, or
// query "?save=true" for the raw-YAML form), the report is also persisted
// to their account.
func (s *Server) handleLint(w http.ResponseWriter, r *http.Request) {
	var manifestYAML, cloud string
	var save, noCost bool

	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var req lintJSONRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxManifestBytes)).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		manifestYAML, cloud, save, noCost = req.Manifest, req.Cloud, req.Save, req.NoCost
	} else {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxManifestBytes))
		if err != nil {
			http.Error(w, "reading request body", http.StatusBadRequest)
			return
		}
		manifestYAML = string(body)
		cloud = r.URL.Query().Get("cloud")
		save = r.URL.Query().Get("save") == "true"
		noCost = r.URL.Query().Get("noCost") == "true"
	}

	if strings.TrimSpace(manifestYAML) == "" {
		http.Error(w, "manifest is empty", http.StatusBadRequest)
		return
	}
	if cloud == "" {
		cloud = "aws"
	}

	// Anonymous calls are only subject to the per-IP rate limit (see
	// lintRateLimiter). Logged-in calls additionally count against their
	// plan's monthly quota (billing.MonthlyCheckLimit) — checked here,
	// before doing any work, so a call over quota doesn't even parse.
	var loggedInUserID int64
	var trackUsage bool
	if s.Store != nil {
		if userID, ok := s.bearerUserID(r); ok {
			sub, err := s.Store.GetSubscription(r.Context(), userID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			limit := planMonthlyLimit(sub.Plan)
			if limit > 0 {
				used, err := s.Store.GetMonthlyUsage(r.Context(), userID)
				if err != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				if used >= limit {
					http.Error(w, "monthly check limit reached for your plan; upgrade or wait until next month", http.StatusTooManyRequests)
					return
				}
			}
			loggedInUserID, trackUsage = userID, true
		}
	}

	resources, err := parser.Parse(strings.NewReader(manifestYAML), "upload.yaml")
	if err != nil {
		http.Error(w, "parsing manifest: "+err.Error(), http.StatusBadRequest)
		return
	}

	violations := rules.RunAll(resources)

	var estimatePtr *cost.Estimate
	if !noCost {
		estimate, err := cost.EstimateManifest(resources, cloud)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		estimatePtr = &estimate
	}
	rep := report.New(violations, estimatePtr)

	if trackUsage {
		// Best-effort: counted after a successful lint, so a transient DB
		// hiccup here doesn't turn into a failed response for the caller.
		_, _ = s.Store.IncrementMonthlyUsage(r.Context(), loggedInUserID)

		if save {
			if resultJSON, err := json.Marshal(rep); err == nil {
				hash := sha256.Sum256([]byte(manifestYAML))
				_, _ = s.Store.CreateReport(r.Context(), loggedInUserID, hex.EncodeToString(hash[:]), resultJSON)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = rep.WriteJSON(w)
}
