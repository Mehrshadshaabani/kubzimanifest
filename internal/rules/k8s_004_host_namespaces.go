package rules

import (
	"strings"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-004"
		title    = "Host namespace sharing enabled"
		severity = SeverityCritical
		docLink  = "https://mflint.dev/rules/K8S-004"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		spec := wl.PodTemplate.Spec
		var flags []string
		if spec.HostNetwork {
			flags = append(flags, "hostNetwork")
		}
		if spec.HostPID {
			flags = append(flags, "hostPID")
		}
		if spec.HostIPC {
			flags = append(flags, "hostIPC")
		}
		if len(flags) == 0 {
			return nil
		}
		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "pod spec enables " + strings.Join(flags, ", ") + ", breaking isolation from the host node",
			Suggestion: "Remove hostNetwork/hostPID/hostIPC unless the workload has a specific, documented need for host namespace access.",
		}}
	}))
}
