package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Claims represents JWT token claims for benchmarking
type Claims struct {
	Username  string `json:"username"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

// BenchmarkJWTGeneration benchmarks JWT token generation
func BenchmarkJWTGeneration(b *testing.B) {
	username := "testuser"
	secret := []byte("test-jwt-secret-key-32-bytes-long")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		now := time.Now().Unix()

		// Create JWT header
		header := map[string]string{
			"alg": "HS256",
			"typ": "JWT",
		}
		headerJSON, _ := json.Marshal(header)
		headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

		// Create JWT payload
		payload := Claims{
			Username:  username,
			ExpiresAt: now + 86400, // 24 hours
			IssuedAt:  now,
		}
		payloadJSON, _ := json.Marshal(payload)
		payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

		// Create signature
		message := headerB64 + "." + payloadB64
		h := hmac.New(sha256.New, secret)
		h.Write([]byte(message))
		signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

		// Combine to form JWT
		token := message + "." + signature
		_ = token
	}
}

// BenchmarkJWTValidation benchmarks JWT token validation
func BenchmarkJWTValidation(b *testing.B) {
	secret := []byte("test-jwt-secret-key-32-bytes-long")

	// Pre-generate a valid token
	now := time.Now().Unix()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	payload := Claims{
		Username:  "testuser",
		ExpiresAt: now + 86400,
		IssuedAt:  now,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	message := headerB64 + "." + payloadB64
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	token := message + "." + signature

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Split token
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			b.Fatal("invalid token format")
		}

		// Verify signature
		message := parts[0] + "." + parts[1]
		h := hmac.New(sha256.New, secret)
		h.Write([]byte(message))
		expectedSig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

		isValid := hmac.Equal([]byte(expectedSig), []byte(parts[2]))

		if isValid {
			// Decode payload
			payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				b.Fatal(err)
			}

			var claims Claims
			err = json.Unmarshal(payloadBytes, &claims)
			if err != nil {
				b.Fatal(err)
			}

			// Check expiration
			_ = claims.ExpiresAt > time.Now().Unix()
		}
	}
}

// BenchmarkPasswordValidation benchmarks password validation
func BenchmarkPasswordValidation(b *testing.B) {
	testPasswords := []struct {
		password string
		expected string
		valid    bool
	}{
		{"correctpassword", "correctpassword", true},
		{"wrongpassword", "correctpassword", false},
		{"admin123", "admin123", true},
		{"", "admin123", false},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tc := range testPasswords {
			// Simple constant-time comparison simulation
			isValid := tc.password == tc.expected
			_ = isValid
		}
	}
}

// BenchmarkTokenExpiration benchmarks token expiration checking
func BenchmarkTokenExpiration(b *testing.B) {
	now := time.Now().Unix()

	tokens := []struct {
		expiresAt int64
		isExpired bool
	}{
		{now + 3600, false},  // Expires in 1 hour
		{now - 3600, true},   // Expired 1 hour ago
		{now + 86400, false}, // Expires in 24 hours
		{now - 1, true},      // Just expired
		{now + 1, false},     // Expires in 1 second
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		currentTime := time.Now().Unix()

		for _, token := range tokens {
			isExpired := token.expiresAt <= currentTime
			_ = isExpired
		}
	}
}

// BenchmarkHMACSignature benchmarks HMAC-SHA256 signature generation
func BenchmarkHMACSignature(b *testing.B) {
	secret := []byte("test-jwt-secret-key-32-bytes-long")
	message := []byte("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwiZXhwIjoxNzA0MzY5NjAwfQ")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h := hmac.New(sha256.New, secret)
		h.Write(message)
		signature := h.Sum(nil)
		_ = base64.RawURLEncoding.EncodeToString(signature)
	}
}

// BenchmarkBase64Encoding benchmarks base64 URL encoding
func BenchmarkBase64Encoding(b *testing.B) {
	data := []byte(`{"username":"testuser","exp":1704369600,"iat":1704283200}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		encoded := base64.RawURLEncoding.EncodeToString(data)
		_ = encoded
	}
}

// BenchmarkBase64Decoding benchmarks base64 URL decoding
func BenchmarkBase64Decoding(b *testing.B) {
	encoded := "eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwiZXhwIjoxNzA0MzY5NjAwLCJpYXQiOjE3MDQyODMyMDB9"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			b.Fatal(err)
		}
		_ = decoded
	}
}

// BenchmarkJSONMarshalClaims benchmarks JSON marshaling of JWT claims
func BenchmarkJSONMarshalClaims(b *testing.B) {
	claims := Claims{
		Username:  "testuser",
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(claims)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJSONUnmarshalClaims benchmarks JSON unmarshaling of JWT claims
func BenchmarkJSONUnmarshalClaims(b *testing.B) {
	claimsJSON := []byte(`{"username":"testuser","exp":1704369600,"iat":1704283200}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var claims Claims
		err := json.Unmarshal(claimsJSON, &claims)
		if err != nil {
			b.Fatal(err)
		}
		_ = claims
	}
}

// BenchmarkTokenParsing benchmarks parsing JWT token string
func BenchmarkTokenParsing(b *testing.B) {
	token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6InRlc3R1c2VyIiwiZXhwIjoxNzA0MzY5NjAwLCJpYXQiOjE3MDQyODMyMDB9.signature_here"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			b.Fatal("invalid token")
		}

		header := parts[0]
		payload := parts[1]
		signature := parts[2]

		_ = header
		_ = payload
		_ = signature
	}
}
