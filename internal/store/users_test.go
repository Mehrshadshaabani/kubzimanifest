package store_test

import (
	"context"
	"testing"
	"time"
)

func TestUpsertOAuthUserLinksByEmail(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	suffix := time.Now().Format("20060102150405.000000000")
	email := "oauth+" + suffix + "@example.com"
	githubID := "gh-" + suffix
	googleID := "google-" + suffix

	viaGitHub, err := db.UpsertGitHubUser(ctx, githubID, email)
	if err != nil {
		t.Fatalf("UpsertGitHubUser: %v", err)
	}
	if viaGitHub.ID == 0 {
		t.Fatal("expected a created user id")
	}

	// Same email via Google should link onto the same account, not create a
	// second one.
	viaGoogle, err := db.UpsertGoogleUser(ctx, googleID, email)
	if err != nil {
		t.Fatalf("UpsertGoogleUser: %v", err)
	}
	if viaGoogle.ID != viaGitHub.ID {
		t.Errorf("UpsertGoogleUser created a different user (id=%d) than UpsertGitHubUser (id=%d) for the same email", viaGoogle.ID, viaGitHub.ID)
	}

	// Calling UpsertGitHubUser again for the same github id is idempotent.
	again, err := db.UpsertGitHubUser(ctx, githubID, email)
	if err != nil {
		t.Fatalf("UpsertGitHubUser (again): %v", err)
	}
	if again.ID != viaGitHub.ID {
		t.Errorf("repeated UpsertGitHubUser changed the user id: got %d, want %d", again.ID, viaGitHub.ID)
	}
}
