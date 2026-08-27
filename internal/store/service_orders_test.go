package store_test

import (
	"context"
	"testing"

	"mflint/internal/store"
)

func TestServiceOrderAndCheckoutSessionRoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, "svcorder@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	order, err := db.CreateServiceOrder(ctx, store.ServiceOrder{
		UserID:       user.ID,
		ServiceID:    "k8s-cloud",
		PackageID:    "growth",
		ServiceName:  "Kubernetes Cloud Setup & Migration",
		PackageName:  "Growth",
		PriceUSD:     1499,
		ContactName:  "Jane Doe",
		ContactEmail: "jane@example.com",
		ProjectNotes: "Need HA cluster on GKE",
	})
	if err != nil {
		t.Fatalf("CreateServiceOrder: %v", err)
	}
	if order.Status != "pending_payment" {
		t.Errorf("expected status pending_payment, got %q", order.Status)
	}

	orders, err := db.ListServiceOrders(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListServiceOrders: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != order.ID {
		t.Fatalf("expected 1 order matching %d, got %+v", order.ID, orders)
	}

	cs, err := db.CreateServiceCheckoutSession(ctx, user.ID, order.ID, "nowpayments", "svc-order-abc")
	if err != nil {
		t.Fatalf("CreateServiceCheckoutSession: %v", err)
	}
	if cs.Status != "pending" {
		t.Errorf("expected pending status, got %q", cs.Status)
	}

	got, err := db.GetServiceCheckoutSessionByOrderID(ctx, "svc-order-abc")
	if err != nil {
		t.Fatalf("GetServiceCheckoutSessionByOrderID: %v", err)
	}
	if got.ServiceOrderID != order.ID {
		t.Errorf("expected service order id %d, got %d", order.ID, got.ServiceOrderID)
	}

	if err := db.UpdateServiceCheckoutSessionStatus(ctx, "svc-order-abc", "completed"); err != nil {
		t.Fatalf("UpdateServiceCheckoutSessionStatus: %v", err)
	}
	if err := db.UpdateServiceOrderStatus(ctx, order.ID, "paid"); err != nil {
		t.Fatalf("UpdateServiceOrderStatus: %v", err)
	}

	orders, err = db.ListServiceOrders(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListServiceOrders after update: %v", err)
	}
	if orders[0].Status != "paid" {
		t.Errorf("expected status paid after update, got %q", orders[0].Status)
	}

	if _, err := db.GetServiceCheckoutSessionByOrderID(ctx, "no-such-order"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for unknown order id, got %v", err)
	}
}
