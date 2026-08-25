package cost_test

import (
	"strings"
	"testing"

	"mflint/internal/cost"
	"mflint/internal/parser"
)

func TestEstimateManifestCompute(t *testing.T) {
	input := `
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
          resources:
            requests:
              cpu: "1"
              memory: 1Gi
            limits:
              cpu: "2"
              memory: 2Gi
`
	resources, err := parser.Parse(strings.NewReader(input), "test.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	est, err := cost.EstimateManifest(resources, "aws")
	if err != nil {
		t.Fatalf("EstimateManifest: %v", err)
	}
	if len(est.Workloads) != 1 {
		t.Fatalf("expected 1 workload cost, got %d", len(est.Workloads))
	}
	w := est.Workloads[0]

	pricing, _ := cost.PricingFor("aws")
	// 1 vCPU + 1Gi request, 2 replicas, at the embedded AWS rate.
	wantLow := round2((1*pricing.ComputePricePerVCPUHour + 1*pricing.ComputePricePerGiBMemoryHour) * pricing.HoursPerMonth * 2)
	wantHigh := round2((2*pricing.ComputePricePerVCPUHour + 2*pricing.ComputePricePerGiBMemoryHour) * pricing.HoursPerMonth * 2)

	if w.MonthlyLowUSD != wantLow {
		t.Errorf("MonthlyLowUSD = %v, want %v", w.MonthlyLowUSD, wantLow)
	}
	if w.MonthlyHighUSD != wantHigh {
		t.Errorf("MonthlyHighUSD = %v, want %v", w.MonthlyHighUSD, wantHigh)
	}
	if w.MonthlyHighUSD < w.MonthlyLowUSD {
		t.Errorf("high estimate %v should be >= low estimate %v", w.MonthlyHighUSD, w.MonthlyLowUSD)
	}
	if est.TotalMonthlyLowUSD != w.MonthlyLowUSD {
		t.Errorf("TotalMonthlyLowUSD = %v, want %v", est.TotalMonthlyLowUSD, w.MonthlyLowUSD)
	}
}

func TestEstimateManifestStorage(t *testing.T) {
	input := `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data
spec:
  storageClassName: gp3
  resources:
    requests:
      storage: 100Gi
`
	resources, err := parser.Parse(strings.NewReader(input), "test.yaml")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	est, err := cost.EstimateManifest(resources, "aws")
	if err != nil {
		t.Fatalf("EstimateManifest: %v", err)
	}
	if len(est.Storage) != 1 {
		t.Fatalf("expected 1 storage cost, got %d", len(est.Storage))
	}
	if est.Storage[0].StorageClass != "gp3" {
		t.Errorf("StorageClass = %q, want gp3", est.Storage[0].StorageClass)
	}
	if est.Storage[0].MonthlyLowUSD <= 0 {
		t.Errorf("expected positive storage cost, got %v", est.Storage[0].MonthlyLowUSD)
	}
}

func TestEstimateManifestUnknownCloud(t *testing.T) {
	if _, err := cost.EstimateManifest(nil, "digitalocean"); err == nil {
		t.Fatal("expected error for unsupported cloud")
	}
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
