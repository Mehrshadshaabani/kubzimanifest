package rules

import (
	appsv1 "k8s.io/api/apps/v1"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-009"
		title    = "No replica count set"
		severity = SeverityWarning
		docLink  = "https://mflint.dev/rules/K8S-009"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		dep, ok := target.Object.(*appsv1.Deployment)
		if !ok || dep.Spec.Replicas != nil {
			return nil
		}
		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "spec.replicas is not set, so it defaults to 1 and the workload has no high availability",
			Suggestion: "Set spec.replicas explicitly (2 or more for HA), even if the value is 1 for a genuinely single-instance workload.",
		}}
	}))
}
