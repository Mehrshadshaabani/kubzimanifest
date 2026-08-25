package store_test

import (
	"context"
	"testing"
	"time"

	"mflint/internal/store"
)

func TestAPIKeyLifecycle(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	user, err := db.CreateUser(ctx, uniqueEmail(t), "hash")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	hash := "hash-" + time.Now().Format("20060102150405.000000000")
	created, err := db.CreateAPIKey(ctx, user.ID, "ci", hash)
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if created.Label != "ci" {
		t.Errorf("Label = %q, want ci", created.Label)
	}
	if created.LastUsedAt != nil {
		t.Errorf("expected LastUsedAt nil for a fresh key, got %v", created.LastUsedAt)
	}

	gotUserID, err := db.GetUserIDByAPIKeyHash(ctx, hash)
	if err != nil {
		t.Fatalf("GetUserIDByAPIKeyHash: %v", err)
	}
	if gotUserID != user.ID {
		t.Errorf("GetUserIDByAPIKeyHash = %d, want %d", gotUserID, user.ID)
	}

	db.TouchAPIKeyLastUsed(ctx, hash)

	keys, err := db.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	if len(keys) != 1 || keys[0].ID != created.ID {
		t.Fatalf("ListAPIKeys = %+v, want exactly the created key", keys)
	}
	if keys[0].LastUsedAt == nil {
		t.Error("expected LastUsedAt to be set after TouchAPIKeyLastUsed")
	}

	if err := db.DeleteAPIKey(ctx, user.ID, created.ID); err != nil {
		t.Fatalf("DeleteAPIKey: %v", err)
	}
	if _, err := db.GetUserIDByAPIKeyHash(ctx, hash); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}

	if err := db.DeleteAPIKey(ctx, user.ID, created.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound deleting an already-deleted key, got %v", err)
	}
}
