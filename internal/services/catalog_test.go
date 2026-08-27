package services_test

import (
	"testing"

	"mflint/internal/services"
)

func TestFind(t *testing.T) {
	svc, pkg, ok := services.Find("k8s-cloud", "growth")
	if !ok {
		t.Fatal("expected k8s-cloud/growth to be found")
	}
	if svc.ID != "k8s-cloud" || pkg.ID != "growth" || pkg.PriceUSD != 1499 {
		t.Fatalf("unexpected result: %+v %+v", svc, pkg)
	}

	if _, _, ok := services.Find("nope", "starter"); ok {
		t.Fatal("expected unknown service to not be found")
	}
	if _, _, ok := services.Find("k8s-cloud", "nope"); ok {
		t.Fatal("expected unknown package to not be found")
	}
}

// Every catalog entry needs a positive price unless it's explicitly a
// Custom/contact-us package, since a zero-price non-custom package would
// silently create a free checkout.
func TestCatalogPricesAreSane(t *testing.T) {
	for _, svc := range services.Catalog {
		if len(svc.Packages) == 0 {
			t.Errorf("service %q has no packages", svc.ID)
		}
		for _, pkg := range svc.Packages {
			if pkg.Custom {
				if pkg.PriceUSD != 0 {
					t.Errorf("%s/%s: custom package should not set PriceUSD", svc.ID, pkg.ID)
				}
				continue
			}
			if pkg.PriceUSD <= 0 {
				t.Errorf("%s/%s: non-custom package must have a positive PriceUSD", svc.ID, pkg.ID)
			}
			if pkg.Billing != "one-time" && pkg.Billing != "/mo" {
				t.Errorf("%s/%s: unexpected billing %q", svc.ID, pkg.ID, pkg.Billing)
			}
		}
	}
}
