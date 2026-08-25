package rules

import (
	"fmt"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-006"
		title    = "Missing liveness/readiness probe"
		severity = SeverityWarning
		docLink  = "https://mflint.dev/rules/K8S-006"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		var out []Violation
		for _, c := range wl.PodTemplate.Spec.Containers {
			var missing []string
			if c.LivenessProbe == nil {
				missing = append(missing, "livenessProbe")
			}
			if c.ReadinessProbe == nil {
				missing = append(missing, "readinessProbe")
			}
			if len(missing) == 0 {
				continue
			}
			out = append(out, Violation{
				RuleID:     id,
				Title:      title,
				Severity:   severity,
				DocLink:    docLink,
				Resource:   target.Ref,
				Message:    fmt.Sprintf("container %q is missing: %v", c.Name, missing),
				Suggestion: "Add livenessProbe and readinessProbe so Kubernetes can detect hangs and gate traffic until the container is ready.",
			})
		}
		return out
	}))
}
