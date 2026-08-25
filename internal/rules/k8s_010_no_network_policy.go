package rules

import (
	networkingv1 "k8s.io/api/networking/v1"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-010"
		title    = "No NetworkPolicy restricting pod traffic"
		severity = SeverityWarning
		docLink  = "https://mflint.dev/rules/K8S-010"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, all []parser.Resource) []Violation {
		if _, ok := parser.AsWorkload(target.Object); !ok {
			return nil
		}
		for _, res := range all {
			if _, ok := res.Object.(*networkingv1.NetworkPolicy); ok && res.Ref.Namespace == target.Ref.Namespace {
				return nil
			}
		}
		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "no NetworkPolicy found in this namespace, so pod traffic is unrestricted by default",
			Suggestion: "Add a NetworkPolicy scoping ingress/egress traffic for this namespace, even a default-deny baseline plus explicit allows.",
		}}
	}))
}
