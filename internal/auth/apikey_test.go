package auth_test

import (
	"testing"

	"mflint/internal/auth"
)

func TestGenerateAPIKey(t *testing.T) {
	raw, hash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if !auth.IsAPIKey(raw) {
		t.Errorf("IsAPIKey(%q) = false, want true", raw)
	}
	if hash != auth.HashAPIKey(raw) {
		t.Error("returned hash does not match HashAPIKey(raw)")
	}

	raw2, hash2, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey (second): %v", err)
	}
	if raw == raw2 || hash == hash2 {
		t.Error("two generated keys should never collide")
	}
}

func TestIsAPIKeyDistinguishesFromJWT(t *testing.T) {
	if auth.IsAPIKey("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1aWQiOjF9.sig") {
		t.Error("a JWT should not be identified as an API key")
	}
}
