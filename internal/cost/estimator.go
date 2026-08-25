// Package cost estimates monthly cloud spend from a manifest set's resource
// requests/limits. It never calls a live pricing API: rates come from the
// embedded static tables in pricing_*.json, and every result is a range
// clearly framed as an estimate rather than a guaranteed bill.
package cost

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"mflint/internal/parser"
)

// WorkloadCost is the estimated monthly compute cost for one Deployment/
// StatefulSet/DaemonSet/Pod, across all its replicas.
type WorkloadCost struct {
	Resource       parser.ResourceRef `json:"resource"`
	Replicas       int32              `json:"replicas"`
	MonthlyLowUSD  float64            `json:"monthlyLowUsd"`
	MonthlyHighUSD float64            `json:"monthlyHighUsd"`
	Note           string             `json:"note,omitempty"`
}

// StorageCost is the estimated monthly cost for one PersistentVolumeClaim.
type StorageCost struct {
	Resource       parser.ResourceRef `json:"resource"`
	StorageClass   string             `json:"storageClass"`
	SizeGB         float64            `json:"sizeGb"`
	MonthlyLowUSD  float64            `json:"monthlyLowUsd"`
	MonthlyHighUSD float64            `json:"monthlyHighUsd"`
}

// Estimate is the full cost report for a manifest set.
type Estimate struct {
	Cloud               string         `json:"cloud"`
	Currency            string         `json:"currency"`
	PricingAsOf         string         `json:"pricingAsOf"`
	Methodology         string         `json:"methodology"`
	Workloads           []WorkloadCost `json:"workloads"`
	Storage             []StorageCost  `json:"storage"`
	TotalMonthlyLowUSD  float64        `json:"totalMonthlyLowUsd"`
	TotalMonthlyHighUSD float64        `json:"totalMonthlyHighUsd"`
}

// EstimateManifest computes a cost Estimate for every workload and PVC in
// resources, priced against the named cloud ("aws", "gcp", or "azure").
func EstimateManifest(resources []parser.Resource, cloud string) (Estimate, error) {
	pricing, ok := PricingFor(cloud)
	if !ok {
		return Estimate{}, fmt.Errorf("cost: unknown cloud %q (supported: %v)", cloud, Clouds())
	}

	est := Estimate{
		Cloud:       cloud,
		Currency:    pricing.Currency,
		PricingAsOf: pricing.AsOf,
		Methodology: pricing.Methodology,
		Workloads:   []WorkloadCost{},
		Storage:     []StorageCost{},
	}

	for _, res := range resources {
		if wl, ok := parser.AsWorkload(res.Object); ok {
			est.Workloads = append(est.Workloads, estimateWorkload(res.Ref, wl, pricing))
			continue
		}
		if pvc, ok := res.Object.(*corev1.PersistentVolumeClaim); ok {
			est.Storage = append(est.Storage, estimateStorage(res.Ref, pvc, pricing))
		}
	}

	for _, w := range est.Workloads {
		est.TotalMonthlyLowUSD += w.MonthlyLowUSD
		est.TotalMonthlyHighUSD += w.MonthlyHighUSD
	}
	for _, s := range est.Storage {
		est.TotalMonthlyLowUSD += s.MonthlyLowUSD
		est.TotalMonthlyHighUSD += s.MonthlyHighUSD
	}
	// Summing already-rounded per-resource costs can leave float64 residue
	// (e.g. 44.18000000000001); round the totals too so the JSON output is clean.
	est.TotalMonthlyLowUSD = round2(est.TotalMonthlyLowUSD)
	est.TotalMonthlyHighUSD = round2(est.TotalMonthlyHighUSD)
	return est, nil
}

func estimateWorkload(ref parser.ResourceRef, wl parser.Workload, pricing Pricing) WorkloadCost {
	var reqCPU, reqMemGiB, limCPU, limMemGiB float64
	for _, c := range parser.AllContainers(wl.PodTemplate.Spec) {
		cReqCPU := vcpuOf(c.Resources.Requests)
		cReqMem := gibOf(c.Resources.Requests)
		cLimCPU := vcpuOf(c.Resources.Limits)
		cLimMem := gibOf(c.Resources.Limits)

		reqCPU += cReqCPU
		reqMemGiB += cReqMem

		// "High" end of the estimate uses limits where set (worst case
		// actual usage); falls back to the request when no limit is set.
		if cLimCPU > 0 {
			limCPU += cLimCPU
		} else {
			limCPU += cReqCPU
		}
		if cLimMem > 0 {
			limMemGiB += cLimMem
		} else {
			limMemGiB += cReqMem
		}
	}

	replicas := float64(wl.Replicas)
	low := (reqCPU*pricing.ComputePricePerVCPUHour + reqMemGiB*pricing.ComputePricePerGiBMemoryHour) * pricing.HoursPerMonth * replicas
	high := (limCPU*pricing.ComputePricePerVCPUHour + limMemGiB*pricing.ComputePricePerGiBMemoryHour) * pricing.HoursPerMonth * replicas

	wc := WorkloadCost{
		Resource:       ref,
		Replicas:       wl.Replicas,
		MonthlyLowUSD:  round2(low),
		MonthlyHighUSD: round2(high),
	}
	if reqCPU == 0 && reqMemGiB == 0 && limCPU == 0 && limMemGiB == 0 {
		wc.Note = "no cpu/memory requests or limits set; cost cannot be estimated (see rule K8S-005)"
	}
	return wc
}

func estimateStorage(ref parser.ResourceRef, pvc *corev1.PersistentVolumeClaim, pricing Pricing) StorageCost {
	sizeGB := 0.0
	if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		sizeGB = float64(q.Value()) / 1e9
	}
	class := ""
	if pvc.Spec.StorageClassName != nil {
		class = *pvc.Spec.StorageClassName
	}
	monthly := round2(sizeGB * pricing.storageRate(class))
	return StorageCost{
		Resource:       ref,
		StorageClass:   class,
		SizeGB:         round2(sizeGB),
		MonthlyLowUSD:  monthly,
		MonthlyHighUSD: monthly,
	}
}

func vcpuOf(list corev1.ResourceList) float64 {
	q, ok := list[corev1.ResourceCPU]
	if !ok {
		return 0
	}
	return float64(q.MilliValue()) / 1000.0
}

func gibOf(list corev1.ResourceList) float64 {
	q, ok := list[corev1.ResourceMemory]
	if !ok {
		return 0
	}
	return float64(q.Value()) / (1024 * 1024 * 1024)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
