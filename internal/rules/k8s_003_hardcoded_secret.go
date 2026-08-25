package rules

import (
	"fmt"
	"strings"

	"mflint/internal/parser"
)

// secretLikeEnvNames are substrings that, when found in an env var name,
// suggest the value should come from a Secret rather than a plain literal.
var secretLikeEnvNames = []string{
	"SECRET", "PASSWORD", "PASSWD", "TOKEN", "APIKEY", "API_KEY",
	"PRIVATE_KEY", "CREDENTIAL", "ACCESS_KEY", "AUTH",
}

func looksLikeSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, needle := range secretLikeEnvNames {
		if strings.Contains(upper, needle) {
			return true
		}
	}
	return false
}

func init() {
	const (
		id       = "K8S-003"
		title    = "Secret-like value hardcoded in env var"
		severity = SeverityCritical
		docLink  = "https://mflint.dev/rules/K8S-003"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		wl, ok := parser.AsWorkload(target.Object)
		if !ok {
			return nil
		}
		var out []Violation
		for _, c := range parser.AllContainers(wl.PodTemplate.Spec) {
			for _, env := range c.Env {
				if env.Value != "" && env.ValueFrom == nil && looksLikeSecretName(env.Name) {
					out = append(out, Violation{
						RuleID:     id,
						Title:      title,
						Severity:   severity,
						DocLink:    docLink,
						Resource:   target.Ref,
						Message:    fmt.Sprintf("container %q sets env var %q to a hardcoded value instead of referencing a Secret", c.Name, env.Name),
						Suggestion: "Reference a Secret instead: env.valueFrom.secretKeyRef, or envFrom.secretRef.",
					})
				}
			}
		}
		return out
	}))
}
