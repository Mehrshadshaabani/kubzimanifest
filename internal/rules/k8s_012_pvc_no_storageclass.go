package rules

import (
	corev1 "k8s.io/api/core/v1"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-012"
		title    = "PVC without storageClassName"
		severity = SeverityInfo
		docLink  = "https://mflint.dev/rules/K8S-012"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, _ []parser.Resource) []Violation {
		pvc, ok := target.Object.(*corev1.PersistentVolumeClaim)
		if !ok || pvc.Spec.StorageClassName != nil {
			return nil
		}
		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "spec.storageClassName is not set, so the cluster's default StorageClass (or none) will be used implicitly",
			Suggestion: "Set spec.storageClassName explicitly so storage class, and therefore cost and performance, is predictable.",
		}}
	}))
}
