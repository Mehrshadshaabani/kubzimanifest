package rules

import (
	"strings"

	"mflint/internal/parser"
)

var costAllocationLabelKeys = []string{"team", "environment", "env", "owner"}

func hasCostAllocationLabel(labels map[string]string) bool {
	for k := range labels {
		lower := strings.ToLower(k)
		for _, want := range costAllocationLabelKeys {
			if strings.Contains(lower, want) {
				return true
			}
		}
	}
	return false
}

func init() {
	const (
		id       = "K8S-011"
		title    = "No cost allocation labels"
		severity = SeverityInfo
		docLink  = "https://mflint.dev/rules/K8S-011"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok || hasCostAllocationLabel(wl.Labels) {
			return nil
		}
		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "no team/environment/owner label found, so this workload's spend can't be attributed in cost reports",
			Suggestion: "Add labels such as team, environment, and owner for cost allocation and ownership tracking.",
		}}
	}))
}
