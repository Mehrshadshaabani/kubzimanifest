package rules

import (
	"fmt"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-001"
		title    = "Container may run as root"
		severity = SeverityCritical
		docLink  = "https://mflint.dev/rules/K8S-001"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		spec := wl.PodTemplate.Spec
		podLevelNonRoot := spec.SecurityContext != nil && spec.SecurityContext.RunAsNonRoot != nil && *spec.SecurityContext.RunAsNonRoot

		var out []Violation
		for _, c := range parser.AllContainers(spec) {
			effective := podLevelNonRoot
			if c.SecurityContext != nil && c.SecurityContext.RunAsNonRoot != nil {
				effective = *c.SecurityContext.RunAsNonRoot
			}
			if !effective {
				out = append(out, Violation{
					RuleID:     id,
					Title:      title,
					Severity:   severity,
					DocLink:    docLink,
					Resource:   target.Ref,
					Message:    fmt.Sprintf("container %q does not set securityContext.runAsNonRoot (pod- or container-level), so it may run as root", c.Name),
					Suggestion: "Set securityContext.runAsNonRoot: true (and a non-zero runAsUser) at the pod or container level.",
				})
			}
		}
		return out
	}))
}
