package rules_test

import "testing"

// Each case provides a "bad" manifest that must trigger at least one
// violation for ruleID, and a "good" manifest that must trigger none.
func TestRules(t *testing.T) {
	cases := []struct {
		name string
		rule string
		bad  string
		good string
	}{
		{
			name: "root user",
			rule: "K8S-001",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
        - name: web
          image: nginx:1.25.3
`,
		},
		{
			name: "privileged container",
			rule: "K8S-002",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
          securityContext:
            privileged: true
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
          securityContext:
            privileged: false
`,
		},
		{
			name: "hardcoded secret",
			rule: "K8S-003",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
          env:
            - name: DB_PASSWORD
              value: hunter2
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-secret
                  key: password
`,
		},
		{
			name: "host namespaces",
			rule: "K8S-004",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      hostNetwork: true
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
		},
		{
			name: "missing resources",
			rule: "K8S-005",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
`,
		},
		{
			name: "missing probes",
			rule: "K8S-006",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
`,
		},
		{
			name: "missing pdb",
			rule: "K8S-007",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: web-pdb
  namespace: default
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: web
`,
		},
		{
			name: "unpinned image",
			rule: "K8S-008",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
		},
		{
			name: "no replica count",
			rule: "K8S-009",
			bad: `
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
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
		},
		{
			name: "no network policy",
			rule: "K8S-010",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
  namespace: default
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
`,
		},
		{
			name: "missing cost labels",
			rule: "K8S-011",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
    team: platform
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
		},
		{
			name: "pvc no storage class",
			rule: "K8S-012",
			bad: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
`,
			good: `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data
spec:
  storageClassName: standard
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
`,
		},
		{
			name: "no termination grace period",
			rule: "K8S-013",
			bad: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25.3
`,
			good: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      terminationGracePeriodSeconds: 60
      containers:
        - name: web
          image: nginx:1.25.3
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			badResources := parseYAML(t, tc.bad)
			if got := countViolations(t, tc.rule, badResources); got < 1 {
				t.Errorf("%s: expected >=1 violation on bad manifest, got %d", tc.rule, got)
			}

			goodResources := parseYAML(t, tc.good)
			if got := countViolations(t, tc.rule, goodResources); got != 0 {
				t.Errorf("%s: expected 0 violations on good manifest, got %d", tc.rule, got)
			}
		})
	}
}
