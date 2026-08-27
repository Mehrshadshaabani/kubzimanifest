// Package services defines the fixed-price consulting/service catalog shown
// at /services — Kubernetes and DevOps engagements sold alongside the Cubzi
// SaaS product, using the same crypto checkout as plan upgrades
// (internal/billing). The catalog is static Go data, not a DB table: it
// changes by editing this file and redeploying, same pattern as
// billing.MonthlyCheckLimit.
package services

// Package is one pricing tier within a Service. A Custom package has no
// fixed PriceUSD — it's sold as "contact us" instead of going through
// checkout, since its scope (and therefore price) isn't fixed up front.
type Package struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	PriceUSD int      `json:"priceUsd"`
	Billing  string   `json:"billing"` // "one-time" or "/mo"
	Features []string `json:"features"`
	Custom   bool     `json:"custom"`
}

type Service struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Tagline     string    `json:"tagline"`
	Description string    `json:"description"`
	Packages    []Package `json:"packages"`
}

var Catalog = []Service{
	{
		ID:          "k8s-cloud",
		Name:        "Kubernetes Cloud Setup & Migration",
		Tagline:     "Production-ready clusters on EKS, GKE, AKS, or DOKS",
		Description: "We design, provision, and harden a managed Kubernetes cluster on your cloud of choice, and migrate existing workloads onto it with zero-downtime rollout.",
		Packages: []Package{
			{ID: "starter", Name: "Starter", PriceUSD: 699, Billing: "one-time", Features: []string{
				"Single-cluster setup on AWS/GCP/Azure/DigitalOcean",
				"Ingress + automatic TLS",
				"CI/CD hookup for your existing pipeline",
				"Handover docs",
			}},
			{ID: "growth", Name: "Growth", PriceUSD: 1499, Billing: "one-time", Features: []string{
				"Everything in Starter",
				"HA multi-node cluster with autoscaling",
				"Prometheus + Grafana monitoring stack",
				"Secrets management (sealed-secrets / external-secrets)",
				"Zero-downtime migration of existing workloads",
			}},
			{ID: "enterprise", Name: "Enterprise", Custom: true, Features: []string{
				"Multi-cluster / multi-region",
				"Compliance & security hardening",
				"Dedicated support channel",
				"Custom scope — talk to us",
			}},
		},
	},
	{
		ID:          "k8s-onprem",
		Name:        "Kubernetes On-Premises Deployment",
		Tagline:     "Self-hosted clusters on your own hardware",
		Description: "Bare-metal or private-datacenter Kubernetes with kubeadm, k3s, or RKE2 — for teams that need full control over where their data lives.",
		Packages: []Package{
			{ID: "starter", Name: "Starter", PriceUSD: 999, Billing: "one-time", Features: []string{
				"Single on-prem cluster (kubeadm or k3s)",
				"Networking (CNI) + basic ingress",
				"Handover docs",
			}},
			{ID: "growth", Name: "Growth", PriceUSD: 2199, Billing: "one-time", Features: []string{
				"Everything in Starter",
				"HA control-plane across multiple nodes",
				"Persistent storage (Longhorn or Ceph)",
				"Private container registry",
				"Monitoring stack",
			}},
			{ID: "enterprise", Name: "Enterprise", Custom: true, Features: []string{
				"Air-gapped / offline installs",
				"Multi-site clusters",
				"Hardware sizing & procurement guidance",
				"Ongoing SLA — talk to us",
			}},
		},
	},
	{
		ID:          "devops-service",
		Name:        "DevOps as a Service",
		Tagline:     "Ongoing CI/CD, observability, and infra ops",
		Description: "A monthly retainer covering your CI/CD pipelines, infrastructure-as-code, monitoring, and hands-on support — without hiring a full-time DevOps engineer.",
		Packages: []Package{
			{ID: "starter", Name: "Starter", PriceUSD: 490, Billing: "/mo", Features: []string{
				"CI/CD pipeline setup & maintenance",
				"Infrastructure as code (Terraform)",
				"Basic monitoring/alerting",
				"Up to 10 support hours/month",
			}},
			{ID: "growth", Name: "Growth", PriceUSD: 990, Billing: "/mo", Features: []string{
				"Everything in Starter",
				"Full observability stack (metrics, logs, traces)",
				"GitOps (ArgoCD/Flux)",
				"Incident response",
				"Up to 25 support hours/month",
			}},
			{ID: "enterprise", Name: "Enterprise", Custom: true, Features: []string{
				"Dedicated DevOps engineer",
				"24/7 on-call coverage",
				"Unlimited support hours — talk to us",
			}},
		},
	},
	{
		ID:          "architecture-design",
		Name:        "Scalable Software Architecture Design",
		Tagline:     "System design for growth, before you build it",
		Description: "A review of your current system (or a from-scratch design) covering service boundaries, data modeling, and a concrete plan for scaling as load grows.",
		Packages: []Package{
			{ID: "starter", Name: "Starter", PriceUSD: 590, Billing: "one-time", Features: []string{
				"Architecture review of your current system",
				"Written report + prioritized roadmap",
			}},
			{ID: "growth", Name: "Growth", PriceUSD: 1990, Billing: "one-time", Features: []string{
				"Full system design from scratch or major redesign",
				"Service boundaries + data model",
				"Scaling plan (caching, queues, sharding)",
				"2 live review sessions",
			}},
			{ID: "enterprise", Name: "Enterprise", Custom: true, Features: []string{
				"Hands-on build partnership",
				"Ongoing architecture advisory",
				"Custom scope — talk to us",
			}},
		},
	},
}

// Find looks up a package by service+package ID, e.g. for validating an
// order request against the catalog before creating a checkout.
func Find(serviceID, packageID string) (Service, Package, bool) {
	for _, svc := range Catalog {
		if svc.ID != serviceID {
			continue
		}
		for _, pkg := range svc.Packages {
			if pkg.ID == packageID {
				return svc, pkg, true
			}
		}
	}
	return Service{}, Package{}, false
}
