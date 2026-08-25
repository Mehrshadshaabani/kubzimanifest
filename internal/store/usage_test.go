package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"mflint/internal/store"
)

// openTestStore connects to Postgres via TEST_DATABASE_URL (or DATABASE_URL
// as a fallback) and runs migrations. Skips the test entirely if neither is
// set, since these are integration tests that need a real database (e.g.
// `docker compose up -d postgres`).
func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL/DATABASE_URL not set; skipping Postgres-backed test")
	}

	db, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestMonthlyUsageIncrement(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, uniqueEmail(t), "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if used, err := db.GetMonthlyUsage(ctx, user.ID); err != nil || used != 0 {
		t.Fatalf("GetMonthlyUsage before any increment = (%d, %v), want (0, nil)", used, err)
	}

	for i := 1; i <= 3; i++ {
		count, err := db.IncrementMonthlyUsage(ctx, user.ID)
		if err != nil {
			t.Fatalf("IncrementMonthlyUsage: %v", err)
		}
		if count != i {
			t.Errorf("IncrementMonthlyUsage call %d returned count %d, want %d", i, count, i)
		}
	}

	used, err := db.GetMonthlyUsage(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetMonthlyUsage: %v", err)
	}
	if used != 3 {
		t.Errorf("final GetMonthlyUsage = %d, want 3", used)
	}
}

func TestCheckoutSessionRoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, uniqueEmail(t), "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	orderID := "mflint_test_" + time.Now().Format("20060102150405.000000000")
	created, err := db.CreateCheckoutSession(ctx, user.ID, "team", "nowpayments", orderID)
	if err != nil {
		t.Fatalf("CreateCheckoutSession: %v", err)
	}
	if created.Status != "pending" {
		t.Errorf("new checkout session status = %q, want pending", created.Status)
	}

	got, err := db.GetCheckoutSessionByOrderID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetCheckoutSessionByOrderID: %v", err)
	}
	if got.UserID != user.ID || got.Plan != "team" {
		t.Errorf("got session %+v, want user=%d plan=team", got, user.ID)
	}

	if err := db.UpdateCheckoutSessionStatus(ctx, orderID, "completed"); err != nil {
		t.Fatalf("UpdateCheckoutSessionStatus: %v", err)
	}
	got, err = db.GetCheckoutSessionByOrderID(ctx, orderID)
	if err != nil {
		t.Fatalf("GetCheckoutSessionByOrderID after update: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("status after update = %q, want completed", got.Status)
	}

	if _, err := db.GetCheckoutSessionByOrderID(ctx, "no-such-order"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for unknown order id, got %v", err)
	}
}

func uniqueEmail(t *testing.T) string {
	t.Helper()
	return "test+" + time.Now().Format("20060102150405.000000000") + "@example.com"
}
