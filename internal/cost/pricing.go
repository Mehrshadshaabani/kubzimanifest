package cost

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed pricing_aws.json
var awsPricingJSON []byte

//go:embed pricing_gcp.json
var gcpPricingJSON []byte

//go:embed pricing_azure.json
var azurePricingJSON []byte

// Pricing is a static, periodically-updated table of on-demand list prices
// for one cloud. These are blended general-purpose compute rates (not tied
// to a specific instance shape), the same simplification cost tools like
// OpenCost use when no live billing API is connected. Treat all output as
// an estimate, not a guaranteed bill.
type Pricing struct {
	Cloud                        string             `json:"cloud"`
	AsOf                         string             `json:"asOf"`
	Currency                     string             `json:"currency"`
	ComputePricePerVCPUHour      float64            `json:"computePricePerVCPUHour"`
	ComputePricePerGiBMemoryHour float64            `json:"computePricePerGiBMemoryHour"`
	HoursPerMonth                float64            `json:"hoursPerMonth"`
	StoragePerGBMonth            map[string]float64 `json:"storagePerGBMonth"`
	// Methodology documents how the two compute rates above were derived,
	// since no cloud provider actually sells vCPU/memory separately for a
	// VM — any such split is necessarily an approximation.
	Methodology string `json:"methodology"`
}

var pricingTables map[string]Pricing

func init() {
	pricingTables = map[string]Pricing{}
	raw := map[string][]byte{
		"aws":   awsPricingJSON,
		"gcp":   gcpPricingJSON,
		"azure": azurePricingJSON,
	}
	for cloud, data := range raw {
		var p Pricing
		if err := json.Unmarshal(data, &p); err != nil {
			panic(fmt.Sprintf("cost: invalid embedded pricing table %q: %v", cloud, err))
		}
		pricingTables[cloud] = p
	}
}

// Clouds returns the supported cloud identifiers, sorted.
func Clouds() []string {
	out := make([]string, 0, len(pricingTables))
	for cloud := range pricingTables {
		out = append(out, cloud)
	}
	sort.Strings(out)
	return out
}

// PricingFor returns the pricing table for a cloud identifier ("aws", "gcp", "azure").
func PricingFor(cloud string) (Pricing, bool) {
	p, ok := pricingTables[cloud]
	return p, ok
}

// storageRate looks up the $/GB/month rate for a storage class, falling back
// to the cloud's "default" rate when the class is unset or unrecognized.
func (p Pricing) storageRate(storageClass string) float64 {
	if storageClass != "" {
		if rate, ok := p.StoragePerGBMonth[storageClass]; ok {
			return rate
		}
	}
	return p.StoragePerGBMonth["default"]
}
