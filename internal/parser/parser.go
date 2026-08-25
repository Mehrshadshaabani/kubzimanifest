// Package parser decodes Kubernetes YAML manifests (single or multi-document)
// into typed objects that internal/rules and internal/cost operate on.
package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"
)

// ResourceRef identifies a decoded resource for reporting purposes.
type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Source    string `json:"source"`
}

func (r ResourceRef) String() string {
	ns := r.Namespace
	if ns == "" {
		ns = "default"
	}
	return fmt.Sprintf("%s/%s (namespace: %s)", r.Kind, r.Name, ns)
}

// Resource pairs a decoded, typed Kubernetes object with its reference info.
// Object is one of: *appsv1.Deployment, *appsv1.StatefulSet, *appsv1.DaemonSet,
// *corev1.Pod, *corev1.Service, *corev1.PersistentVolumeClaim, *corev1.ConfigMap,
// *corev1.Secret, *networkingv1.Ingress, *networkingv1.NetworkPolicy,
// *policyv1.PodDisruptionBudget.
type Resource struct {
	Ref    ResourceRef
	Object interface{}
}

type typeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// ParseDir walks dir for .yaml/.yml files and parses every document in each.
// Files are visited in sorted order so output is deterministic.
func ParseDir(dir string) ([]Resource, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", dir, err)
	}
	sort.Strings(files)

	var all []Resource
	for _, f := range files {
		res, err := ParseFile(f)
		if err != nil {
			return nil, err
		}
		all = append(all, res...)
	}
	return all, nil
}

// ParseFile parses every YAML document in a single file.
func ParseFile(path string) ([]Resource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	return Parse(f, path)
}

// Parse reads one or more YAML documents from r and decodes the supported
// Kubernetes kinds. Unsupported/unknown kinds and empty documents are skipped
// rather than causing an error, so a manifest set can mix resource types
// freely.
func Parse(r io.Reader, source string) ([]Resource, error) {
	reader := k8syaml.NewYAMLReader(bufio.NewReader(r))
	var resources []Resource
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", source, err)
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}

		var tm typeMeta
		if err := sigsyaml.Unmarshal(raw, &tm); err != nil {
			return nil, fmt.Errorf("%s: parsing document header: %w", source, err)
		}
		if tm.Kind == "" {
			continue
		}

		obj, err := decodeTyped(tm.Kind, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: decoding %s %q: %w", source, tm.Kind, tm.Metadata.Name, err)
		}
		if obj == nil {
			// Unsupported kind for this tool's rule set; skip silently.
			continue
		}

		resources = append(resources, Resource{
			Ref: ResourceRef{
				Kind:      tm.Kind,
				Name:      tm.Metadata.Name,
				Namespace: tm.Metadata.Namespace,
				Source:    source,
			},
			Object: obj,
		})
	}
	return resources, nil
}

func decodeTyped(kind string, raw []byte) (interface{}, error) {
	switch kind {
	case "Deployment":
		var o appsv1.Deployment
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "StatefulSet":
		var o appsv1.StatefulSet
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "DaemonSet":
		var o appsv1.DaemonSet
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "Pod":
		var o corev1.Pod
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "Service":
		var o corev1.Service
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "PersistentVolumeClaim":
		var o corev1.PersistentVolumeClaim
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "ConfigMap":
		var o corev1.ConfigMap
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "Secret":
		var o corev1.Secret
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "Ingress":
		var o networkingv1.Ingress
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "NetworkPolicy":
		var o networkingv1.NetworkPolicy
		return &o, sigsyaml.Unmarshal(raw, &o)
	case "PodDisruptionBudget":
		var o policyv1.PodDisruptionBudget
		return &o, sigsyaml.Unmarshal(raw, &o)
	default:
		return nil, nil
	}
}
