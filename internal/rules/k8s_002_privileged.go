package rules

import (
	"fmt"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-002"
		title    = "Privileged container"
		severity = SeverityCritical
		docLink  = "https://mflint.dev/rules/K8S-002"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		var out []Violation
		for _, c := range parser.AllContainers(wl.PodTemplate.Spec) {
			if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
				out = append(out, Violation{
					RuleID:     id,
					Title:      title,
					Severity:   severity,
					DocLink:    docLink,
					Resource:   target.Ref,
					Message:    fmt.Sprintf("container %q sets securityContext.privileged: true, granting it full host access", c.Name),
					Suggestion: "Remove securityContext.privileged. Use specific capabilities (securityContext.capabilities.add) if the container needs elevated access.",
				})
			}
		}
		return out
	}))
}
