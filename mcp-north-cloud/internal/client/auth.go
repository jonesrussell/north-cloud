package client

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// ServiceTokenExpirationHours is the expiration time for service-to-service JWT tokens.
	serviceTokenExpirationHours = 24
)

// AuthenticatedClient wraps an http.Client and adds JWT authentication headers.
type AuthenticatedClient struct {
	client    *http.Client
	jwtSecret string
}

// NewAuthenticatedClient creates a new authenticated HTTP client with default timeout.
// If jwtSecret is empty, requests will be made without authentication.
func NewAuthenticatedClient(jwtSecret string) *AuthenticatedClient {
	return NewAuthenticatedClientWithTimeout(jwtSecret, defaultHTTPTimeout)
}

// NewAuthenticatedClientWithTimeout creates a new authenticated HTTP client with the given timeout.
func NewAuthenticatedClientWithTimeout(jwtSecret string, timeout time.Duration) *AuthenticatedClient {
	return &AuthenticatedClient{
		client: &http.Client{
			Timeout: timeout,
		},
		jwtSecret: jwtSecret,
	}
}

// Do executes an HTTP request with JWT authentication if configured.
func (c *AuthenticatedClient) Do(req *http.Request) (*http.Response, error) {
	if c.jwtSecret != "" {
		token, err := c.generateServiceToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate service token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.client.Do(req)
}

// generateServiceToken generates a JWT token for service-to-service authentication.
func (c *AuthenticatedClient) generateServiceToken() (string, error) {
	if c.jwtSecret == "" {
		return "", errors.New("JWT secret not configured")
	}

	now := time.Now()
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(serviceTokenExpirationHours * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Subject:   "mcp-north-cloud-service",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.jwtSecret))
}

// HTTPClient returns the underlying http.Client for operations that don't need auth.
func (c *AuthenticatedClient) HTTPClient() *http.Client {
	return c.client
}

// GenerateToken generates a JWT token for manual API testing.
// Returns the token string and expiration time.
func (c *AuthenticatedClient) GenerateToken() (token string, expiresAt time.Time, err error) {
	if c.jwtSecret == "" {
		return "", time.Time{}, errors.New("JWT secret not configured")
	}

	now := time.Now()
	expiresAt = now.Add(serviceTokenExpirationHours * time.Hour)
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Subject:   "mcp-north-cloud-cli",
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = jwtToken.SignedString([]byte(c.jwtSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}
	return token, expiresAt, nil
}
