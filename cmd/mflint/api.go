package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mflint/internal/report"
)

// defaultAPIBase is where the CLI talks to when no --api-base/MFLINT_API_BASE
// override and no saved config value are set. Point this at wherever the
// server (cmd/server) is actually deployed before publishing the binary.
const defaultAPIBase = "https://api.mflint.dev"

type usageResponse struct {
	Plan      string `json:"plan"`
	Limit     int    `json:"limit"`
	Used      int    `json:"used"`
	Remaining int    `json:"remaining"`
}

// lintViaAPI sends manifest to the server's /v1/lint and returns the parsed
// report. Every check now runs server-side so the monthly quota enforced on
// the web (billing.MonthlyCheckLimit) applies identically here: anonymous
// calls (no token) are only IP rate-limited, an authenticated call also
// counts against the account's plan.
func lintViaAPI(ctx context.Context, apiBase, token, manifest, cloud string, noCost bool) (*report.Report, error) {
	reqBody, err := json.Marshal(map[string]any{
		"manifest": manifest,
		"cloud":    cloud,
		"noCost":   noCost,
	})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(apiBase, "/") + "/v1/lint"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach mflint API at %s: %w", apiBase, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", apiBase, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		var rep report.Report
		if err := json.Unmarshal(body, &rep); err != nil {
			return nil, fmt.Errorf("unexpected response from %s: %w", apiBase, err)
		}
		return &rep, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("your API key is invalid or expired; run `mflint login` again")
	case http.StatusTooManyRequests:
		return nil, fmt.Errorf("%s\nupgrade your plan at %s/app", strings.TrimSpace(string(body)), apiBase)
	default:
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func fetchUsage(ctx context.Context, apiBase, token string) (*usageResponse, error) {
	url := strings.TrimRight(apiBase, "/") + "/v1/usage"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach mflint API at %s: %w", apiBase, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var u usageResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
