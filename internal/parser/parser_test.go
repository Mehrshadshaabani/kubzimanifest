package parser_test

import (
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	"mflint/internal/parser"
)

func TestParseMultiDocument(t *testing.T) {
	input := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: prod
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: web-config
`
	resources, err := parser.Parse(strings.NewReader(input), "test.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	dep, ok := resources[0].Object.(*appsv1.Deployment)
	if !ok {
		t.Fatalf("expected first resource to decode as *appsv1.Deployment, got %T", resources[0].Object)
	}
	if dep.Name != "web" || dep.Namespace != "prod" {
		t.Errorf("unexpected deployment metadata: name=%q namespace=%q", dep.Name, dep.Namespace)
	}
	if resources[1].Ref.Kind != "ConfigMap" {
		t.Errorf("expected second resource kind ConfigMap, got %q", resources[1].Ref.Kind)
	}
}

func TestParseSkipsUnsupportedKind(t *testing.T) {
	input := `
apiVersion: v1
kind: Namespace
metadata:
  name: unsupported-example
`
	resources, err := parser.Parse(strings.NewReader(input), "test.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected unsupported kind to be skipped, got %d resources", len(resources))
	}
}

func TestAsWorkloadReplicaDefault(t *testing.T) {
	input := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`
	resources, err := parser.Parse(strings.NewReader(input), "test.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	wl, ok := parser.AsWorkload(resources[0].Object)
	if !ok {
		t.Fatalf("expected AsWorkload to succeed for Deployment")
	}
	if wl.Replicas != 1 {
		t.Errorf("expected default replicas 1, got %d", wl.Replicas)
	}
}
