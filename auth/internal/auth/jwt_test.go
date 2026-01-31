package auth_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/auth/internal/auth"
)

func TestNewJWTManager(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		secret     string
		expiration time.Duration
		wantNil    bool
	}{
		{"valid config", "test-secret-key", 24 * time.Hour, false},
		{"empty secret", "", time.Hour, false}, // Still creates manager
		{"zero expiration", "secret", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := auth.NewJWTManager(tt.secret, tt.expiration)
			if (mgr == nil) != tt.wantNil {
				t.Errorf("NewJWTManager() nil = %v, want %v", mgr == nil, tt.wantNil)
			}
		})
	}
}

func TestJWTManager_GenerateToken(t *testing.T) {
	t.Helper()

	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", 24*time.Hour)

	token, err := mgr.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}

	// Token should have 3 parts (header.payload.signature)
	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("GenerateToken() token has %d dots, want 2", parts)
	}
}

func TestJWTManager_ValidateToken_Success(t *testing.T) {
	t.Helper()

	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", 24*time.Hour)

	token, err := mgr.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims == nil {
		t.Fatal("ValidateToken() returned nil claims")
	}

	if claims.Sub != "dashboard" {
		t.Errorf("ValidateToken() sub = %s, want dashboard", claims.Sub)
	}
}

func TestJWTManager_ValidateToken_Expired(t *testing.T) {
	t.Helper()

	// Create manager with very short expiration (negative = already expired)
	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", -time.Hour)

	token, err := mgr.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	_, err = mgr.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() expected error for expired token")
	}
}

func TestJWTManager_ValidateToken_InvalidSignature(t *testing.T) {
	t.Helper()

	mgr1 := auth.NewJWTManager("secret-key-one-32-chars-minimum1", 24*time.Hour)
	mgr2 := auth.NewJWTManager("secret-key-two-32-chars-minimum2", 24*time.Hour)

	// Generate token with mgr1
	token, err := mgr1.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Validate with mgr2 (different secret)
	_, err = mgr2.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() expected error for invalid signature")
	}
}

func TestJWTManager_ValidateToken_MalformedToken(t *testing.T) {
	t.Helper()

	mgr := auth.NewJWTManager("test-secret-key-32-chars-minimum", 24*time.Hour)

	invalidTokens := []string{
		"",
		"not-a-token",
		"only.two.parts.here",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
	}

	for _, token := range invalidTokens {
		_, err := mgr.ValidateToken(token)
		if err == nil {
			t.Errorf("ValidateToken(%q) expected error for malformed token", token)
		}
	}
}
