package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// GenerateCSRFToken creates a 32-byte cryptographically random hex token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: failed to generate CSRF token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidateCSRFToken performs constant-time comparison of cookie and header/form CSRF tokens.
func ValidateCSRFToken(cookieToken, formOrHeaderToken string) bool {
	if cookieToken == "" || formOrHeaderToken == "" {
		return false
	}
	if len(cookieToken) != len(formOrHeaderToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieToken), []byte(formOrHeaderToken)) == 1
}
