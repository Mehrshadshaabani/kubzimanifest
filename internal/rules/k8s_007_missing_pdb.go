package rules

import (
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"mflint/internal/parser"
)

func init() {
	const (
		id       = "K8S-007"
		title    = "Missing PodDisruptionBudget for multi-replica workload"
		severity = SeverityWarning
		docLink  = "https://mflint.dev/rules/K8S-007"
	)
	Register(newRule(id, title, severity, docLink, func(target parser.Resource, all []parser.Resource) []Violation {
		switch target.Object.(type) {
		case *appsv1.Deployment, *appsv1.StatefulSet:
		default:
			return nil
		}
		wl, ok := parser.AsWorkload(target.Object)
		if !ok || wl.Replicas <= 1 {
			return nil
		}

		podLabels := labels.Set(wl.PodTemplate.Labels)
		for _, res := range all {
			pdb, ok := res.Object.(*policyv1.PodDisruptionBudget)
			if !ok || res.Ref.Namespace != target.Ref.Namespace {
				continue
			}
			sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			if err != nil || sel.Empty() {
				continue
			}
			if sel.Matches(podLabels) {
				return nil
			}
		}

		return []Violation{{
			RuleID:     id,
			Title:      title,
			Severity:   severity,
			DocLink:    docLink,
			Resource:   target.Ref,
			Message:    "workload runs multiple replicas but no PodDisruptionBudget in this namespace selects its pods",
			Suggestion: "Add a PodDisruptionBudget (minAvailable or maxUnavailable) with a selector matching this workload's pod labels.",
		}}
	}))
}
