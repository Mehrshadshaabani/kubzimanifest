package rules

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-005"
		title    = "Missing resource requests/limits"
		severity = SeverityWarning
		docLink  = "https://mflint.dev/rules/K8S-005"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		var out []Violation
		for _, c := range parser.AllContainers(wl.PodTemplate.Spec) {
			var missing []string
			if _, hasCPU := c.Resources.Requests[corev1.ResourceCPU]; !hasCPU {
				missing = append(missing, "cpu request")
			}
			if _, hasMem := c.Resources.Requests[corev1.ResourceMemory]; !hasMem {
				missing = append(missing, "memory request")
			}
			if _, hasCPU := c.Resources.Limits[corev1.ResourceCPU]; !hasCPU {
				missing = append(missing, "cpu limit")
			}
			if _, hasMem := c.Resources.Limits[corev1.ResourceMemory]; !hasMem {
				missing = append(missing, "memory limit")
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
				Suggestion: "Set resources.requests and resources.limits for both cpu and memory so the scheduler and cost estimate are accurate.",
			})
		}
		return out
	}))
}
