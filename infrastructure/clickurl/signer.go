// Package clickurl provides HMAC-SHA256 signing and verification for click-tracking URLs.
// Both the search service (sign) and click-tracker service (verify) import this package
// to ensure consistent signature generation and validation.
package clickurl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
)

// SignatureLength is the number of hex characters used for the truncated HMAC signature.
const SignatureLength = 12

// ClickParams holds the parameters that identify a specific click event.
// These fields are combined into a pipe-delimited message for HMAC signing.
type ClickParams struct {
	QueryID        string
	ResultID       string
	Position       int
	Page           int
	Timestamp      int64
	DestinationURL string
}

// Message returns a pipe-delimited string representation of the click parameters
// suitable for HMAC signing. Format: "queryid|resultid|pos|page|timestamp|url".
func (p ClickParams) Message() string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s",
		p.QueryID,
		p.ResultID,
		strconv.Itoa(p.Position),
		strconv.Itoa(p.Page),
		strconv.FormatInt(p.Timestamp, 10),
		p.DestinationURL,
	)
}

// Signer provides HMAC-SHA256 signing and verification using a shared secret.
type Signer struct {
	secret []byte
}

// NewSigner creates a new Signer with the given secret string.
func NewSigner(secret string) *Signer {
	return &Signer{
		secret: []byte(secret),
	}
}

// Sign computes an HMAC-SHA256 of the message and returns the first SignatureLength
// hex characters as the signature.
func (s *Signer) Sign(message string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(message))
	fullHex := hex.EncodeToString(mac.Sum(nil))

	return fullHex[:SignatureLength]
}

// Verify checks whether the given signature matches the HMAC-SHA256 of the message.
// Uses hmac.Equal for constant-time comparison to prevent timing attacks.
func (s *Signer) Verify(message, signature string) bool {
	expected := s.Sign(message)

	return hmac.Equal([]byte(expected), []byte(signature))
}
