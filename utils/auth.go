package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashAuthKey creates a SHA-256 hash of the authorization key for use as a database key
// This avoids storing raw API tokens in the database while maintaining consistent tracking
func HashAuthKey(authHeader string) string {
	if authHeader == "" {
		return "" // Keep empty string as-is for unauthenticated requests
	}
	hash := sha256.Sum256([]byte(authHeader))
	return hex.EncodeToString(hash[:])
}

// MaskAPIKeyForStorage masks an API key for storage alongside the hash
func MaskAPIKeyForStorage(key string) string {
	if key == "" || key == "anonymous" {
		return key
	}

	// Handle "Bearer " prefix
	if len(key) > 7 && key[:7] == "Bearer " {
		token := key[7:] // Remove "Bearer " prefix
		if len(token) <= 4 {
			return "Bearer " + token + "..."
		}
		if len(token) <= 8 {
			return "Bearer " + token[:4] + "..."
		}
		return "Bearer " + token[:4] + "..." + token[len(token)-4:]
	}

	// Handle raw token
	if len(key) <= 4 {
		return key + "..."
	}
	if len(key) <= 8 {
		return key[:4] + "..."
	}
	return key[:4] + "..." + key[len(key)-4:]
}
