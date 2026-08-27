package store_test

import (
	"context"
	"testing"

	"mflint/internal/store"
)

func TestGetUserByID(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, "admin-lookup@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := db.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Email != "admin-lookup@example.com" {
		t.Errorf("expected matching email, got %q", got.Email)
	}

	if _, err := db.GetUserByID(ctx, 999999999); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for unknown id, got %v", err)
	}
}

func TestListAllServiceOrdersIncludesUserEmail(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, "admin-orders@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	order, err := db.CreateServiceOrder(ctx, store.ServiceOrder{
		UserID: user.ID, ServiceID: "k8s-cloud", PackageID: "starter",
		ServiceName: "Kubernetes Cloud Setup & Migration", PackageName: "Starter",
		PriceUSD: 699, ContactName: "A", ContactEmail: "a@a.com",
	})
	if err != nil {
		t.Fatalf("CreateServiceOrder: %v", err)
	}

	all, err := db.ListAllServiceOrders(ctx)
	if err != nil {
		t.Fatalf("ListAllServiceOrders: %v", err)
	}
	var found bool
	for _, o := range all {
		if o.ID == order.ID {
			found = true
			if o.UserEmail != "admin-orders@example.com" {
				t.Errorf("expected joined user email, got %q", o.UserEmail)
			}
		}
	}
	if !found {
		t.Fatal("expected to find created order in ListAllServiceOrders")
	}
}

func TestConsultationRequestRoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	created, err := db.CreateConsultationRequest(ctx, 0, "Jane", "jane@example.com", "Need help scaling")
	if err != nil {
		t.Fatalf("CreateConsultationRequest: %v", err)
	}
	if created.Status != "new" {
		t.Errorf("expected default status 'new', got %q", created.Status)
	}
	if created.UserID != 0 {
		t.Errorf("expected UserID 0 for anonymous request, got %d", created.UserID)
	}

	all, err := db.ListConsultationRequests(ctx)
	if err != nil {
		t.Fatalf("ListConsultationRequests: %v", err)
	}
	var found bool
	for _, c := range all {
		if c.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected to find created consultation request")
	}
}
