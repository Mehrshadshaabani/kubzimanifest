// Package report combines lint violations and a cost estimate into one
// Report, and formats it as a table (for humans/CI logs) or JSON (for
// machine consumption / the API).
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"mflint/internal/cost"
	"mflint/internal/rules"
)

// Report is the full output of a lint run: violations plus (optionally) a
// cost estimate. Cost is a pointer so it can be omitted entirely (e.g. the
// CLI's lint-only mode, or when --cloud is not given).
type Report struct {
	Violations    []rules.Violation `json:"violations"`
	Cost          *cost.Estimate    `json:"cost,omitempty"`
	CriticalCount int               `json:"criticalCount"`
	WarningCount  int               `json:"warningCount"`
	InfoCount     int               `json:"infoCount"`
}

// New builds a Report from violations and an optional cost estimate.
func New(violations []rules.Violation, estimate *cost.Estimate) Report {
	if violations == nil {
		violations = []rules.Violation{}
	}
	r := Report{Violations: violations, Cost: estimate}
	for _, v := range violations {
		switch v.Severity {
		case rules.SeverityCritical:
			r.CriticalCount++
		case rules.SeverityWarning:
			r.WarningCount++
		case rules.SeverityInfo:
			r.InfoCount++
		}
	}
	return r
}

// HasCritical reports whether any CRITICAL violation is present, the signal
// the CLI uses to fail a CI pipeline.
func (r Report) HasCritical() bool {
	return r.CriticalCount > 0
}

// WriteJSON writes the report as indented JSON.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteTable writes a human-readable summary: a violations table grouped by
// severity, then a cost summary if present.
func (r Report) WriteTable(w io.Writer) error {
	if len(r.Violations) == 0 {
		fmt.Fprintln(w, "No violations found.")
	} else {
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "SEVERITY\tRULE\tRESOURCE\tMESSAGE")
		for _, v := range r.Violations {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				strings.ToUpper(string(v.Severity)), v.RuleID, v.Resource.String(), v.Message)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(w, "\n%d critical, %d warning, %d info\n", r.CriticalCount, r.WarningCount, r.InfoCount)
	}

	if r.Cost != nil {
		fmt.Fprintf(w, "\nEstimated monthly cost (%s, list price as of %s, %s) — this is an estimate, not a bill:\n",
			strings.ToUpper(r.Cost.Cloud), r.Cost.PricingAsOf, r.Cost.Currency)
		tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "RESOURCE\tREPLICAS\tLOW\tHIGH")
		for _, wl := range r.Cost.Workloads {
			note := ""
			if wl.Note != "" {
				note = " (" + wl.Note + ")"
			}
			fmt.Fprintf(tw, "%s\t%d\t$%.2f\t$%.2f%s\n", wl.Resource.String(), wl.Replicas, wl.MonthlyLowUSD, wl.MonthlyHighUSD, note)
		}
		for _, s := range r.Cost.Storage {
			fmt.Fprintf(tw, "%s (storage, %.0fGB, %s)\t-\t$%.2f\t$%.2f\n", s.Resource.String(), s.SizeGB, s.StorageClass, s.MonthlyLowUSD, s.MonthlyHighUSD)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(w, "\nTotal: $%.2f - $%.2f / month\n", r.Cost.TotalMonthlyLowUSD, r.Cost.TotalMonthlyHighUSD)
		if r.Cost.Methodology != "" {
			fmt.Fprintf(w, "Pricing methodology: %s\n", r.Cost.Methodology)
		}
	}
	return nil
}
