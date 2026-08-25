package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// APIKeyPrefix marks a bearer token as a long-lived API key rather than a
// short-lived session JWT, so internal/api can tell them apart on sight
// without trying (and failing) to parse one as the other.
const APIKeyPrefix = "mflint_"

// GenerateAPIKey returns a new random API key plus the hash that should be
// stored for it (see HashAPIKey). The raw key is only ever available here —
// callers must show it to the user once and never persist it themselves.
func GenerateAPIKey() (raw, hash string, err error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = APIKeyPrefix + hex.EncodeToString(buf)
	return raw, HashAPIKey(raw), nil
}

// HashAPIKey derives the lookup hash for a raw API key. SHA-256 (not
// bcrypt) is deliberate: an API key is already high-entropy random data,
// not a low-entropy human password, so a fast deterministic hash is both
// sufficient and necessary for an indexed lookup on every request.
func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// IsAPIKey reports whether a bearer token looks like an API key rather
// than a session JWT.
func IsAPIKey(token string) bool {
	return strings.HasPrefix(token, APIKeyPrefix)
}
