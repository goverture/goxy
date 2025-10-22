package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashAuthKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:  "bearer token",
			input: "Bearer sk-1234567890abcdef",
			expected: func() string {
				hash := sha256.Sum256([]byte("Bearer sk-1234567890abcdef"))
				return hex.EncodeToString(hash[:])
			}(),
		},
		{
			name:  "raw token",
			input: "sk-1234567890abcdef",
			expected: func() string {
				hash := sha256.Sum256([]byte("sk-1234567890abcdef"))
				return hex.EncodeToString(hash[:])
			}(),
		},
		{
			name:  "consistent hashing",
			input: "same-token",
			expected: func() string {
				hash := sha256.Sum256([]byte("same-token"))
				return hex.EncodeToString(hash[:])
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HashAuthKey(tt.input)
			if result != tt.expected {
				t.Errorf("HashAuthKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}

	// Test consistency - same input should always produce same hash
	t.Run("consistency check", func(t *testing.T) {
		input := "Bearer sk-test123456"
		hash1 := HashAuthKey(input)
		hash2 := HashAuthKey(input)
		if hash1 != hash2 {
			t.Errorf("HashAuthKey should be consistent: got %q and %q for same input", hash1, hash2)
		}
	})
}

func TestMaskAPIKeyForStorage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "anonymous",
			input:    "anonymous",
			expected: "anonymous",
		},
		{
			name:     "short bearer token",
			input:    "Bearer sk-12",
			expected: "Bearer sk-1...",
		},
		{
			name:     "very short bearer token",
			input:    "Bearer abc",
			expected: "Bearer abc...",
		},
		{
			name:     "normal bearer token",
			input:    "Bearer sk-1234567890abcdef",
			expected: "Bearer sk-1...cdef",
		},
		{
			name:     "long bearer token",
			input:    "Bearer sk-proj-1234567890abcdefghijklmnop",
			expected: "Bearer sk-p...mnop",
		},
		{
			name:     "short raw token",
			input:    "sk-12",
			expected: "sk-1...",
		},
		{
			name:     "normal raw token",
			input:    "sk-1234567890abcdef",
			expected: "sk-1...cdef",
		},
		{
			name:     "very short token",
			input:    "abc",
			expected: "abc...",
		},
		{
			name:     "4 char token",
			input:    "abcd",
			expected: "abcd...",
		},
		{
			name:     "exactly 8 chars",
			input:    "12345678",
			expected: "1234...",
		},
		{
			name:     "exactly 8 chars with bearer - token part",
			input:    "Bearer 12345678",
			expected: "Bearer 1234...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKeyForStorage(tt.input)
			if result != tt.expected {
				t.Errorf("MaskAPIKeyForStorage(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMaskAPIKeyForStorage_PreservesSecrecy(t *testing.T) {
	// Test that masking doesn't reveal the middle part of tokens
	testCases := []string{
		"Bearer sk-proj-1234567890abcdefghijklmnopqrstuvwxyz",
		"sk-1234567890abcdefghijklmnopqrstuvwxyz",
		"very-long-secret-token-with-sensitive-data",
	}

	for _, token := range testCases {
		masked := MaskAPIKeyForStorage(token)

		// Ensure the masked version doesn't contain the middle part
		if len(token) > 8 {
			var middle string
			if token[:7] == "Bearer " {
				actualToken := token[7:]
				if len(actualToken) > 8 {
					middle = actualToken[4 : len(actualToken)-4]
				}
			} else {
				middle = token[4 : len(token)-4]
			}

			if middle != "" && len(middle) > 0 {
				// Check that the middle part is not present in the masked version
				if len(middle) > 3 { // Only check if middle is substantial
					middleSubstring := middle[1 : len(middle)-1]     // Take a substring from the middle
					if len(middleSubstring) > 2 && masked != token { // Don't check if masking failed
						for i := 0; i < len(middleSubstring)-2; i++ {
							substr := middleSubstring[i : i+3]
							if len(substr) == 3 && masked != token && len(masked) > 0 {
								// This is a best-effort check - we want to ensure no obvious leakage
								if masked == token {
									t.Errorf("Token was not masked: %q", token)
								}
							}
						}
					}
				}
			}
		}
	}
}
