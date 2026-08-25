package parser

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Workload is a normalized view over Deployment/StatefulSet/DaemonSet/Pod so
// rules and the cost estimator don't need a type switch each.
type Workload struct {
	PodTemplate corev1.PodTemplateSpec
	Replicas    int32 // always >= 1; DaemonSet reports 1 (per-node, not user-set)
	Labels      map[string]string
	Annotations map[string]string
}

// AsWorkload extracts a normalized Workload view from a Resource's Object,
// if it is one of the pod-scheduling kinds. ok is false for kinds that don't
// schedule pods (Service, Ingress, ConfigMap, Secret, PVC, NetworkPolicy, PDB).
func AsWorkload(obj interface{}) (Workload, bool) {
	switch o := obj.(type) {
	case *appsv1.Deployment:
		replicas := int32(1)
		if o.Spec.Replicas != nil {
			replicas = *o.Spec.Replicas
		}
		return Workload{
			PodTemplate: o.Spec.Template,
			Replicas:    replicas,
			Labels:      o.Labels,
			Annotations: o.Annotations,
		}, true
	case *appsv1.StatefulSet:
		replicas := int32(1)
		if o.Spec.Replicas != nil {
			replicas = *o.Spec.Replicas
		}
		return Workload{
			PodTemplate: o.Spec.Template,
			Replicas:    replicas,
			Labels:      o.Labels,
			Annotations: o.Annotations,
		}, true
	case *appsv1.DaemonSet:
		return Workload{
			PodTemplate: o.Spec.Template,
			Replicas:    1,
			Labels:      o.Labels,
			Annotations: o.Annotations,
		}, true
	case *corev1.Pod:
		return Workload{
			PodTemplate: corev1.PodTemplateSpec{ObjectMeta: o.ObjectMeta, Spec: o.Spec},
			Replicas:    1,
			Labels:      o.Labels,
			Annotations: o.Annotations,
		}, true
	default:
		return Workload{}, false
	}
}

// AllContainers returns spec + init containers together, since most security
// rules apply equally to both.
func AllContainers(spec corev1.PodSpec) []corev1.Container {
	out := make([]corev1.Container, 0, len(spec.InitContainers)+len(spec.Containers))
	out = append(out, spec.InitContainers...)
	out = append(out, spec.Containers...)
	return out
}
