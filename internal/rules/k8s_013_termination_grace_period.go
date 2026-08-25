package rules

import (
	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-013"
		title    = "No terminationGracePeriodSeconds override"
		severity = SeverityInfo
		docLink  = "https://mflint.dev/rules/K8S-013"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok || wl.PodTemplate.Spec.TerminationGracePeriodSeconds != nil {
			return nil
		}
		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "terminationGracePeriodSeconds is not set, so the default 30s applies even if this workload shuts down slower",
			Suggestion: "Set spec.template.spec.terminationGracePeriodSeconds explicitly if the workload needs longer than 30s to drain/shut down cleanly.",
		}}
	}))
}
