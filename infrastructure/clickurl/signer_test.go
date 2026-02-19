package clickurl_test

import (
	"testing"

	"github.com/north-cloud/infrastructure/clickurl"
)

const testSecret = "test-secret-key-for-hmac-signing"

func newTestSigner(t *testing.T) *clickurl.Signer {
	t.Helper()

	return clickurl.NewSigner(testSecret)
}

func TestSign(t *testing.T) {
	signer := newTestSigner(t)
	sig := signer.Sign("test-message")

	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	if len(sig) != clickurl.SignatureLength {
		t.Fatalf("expected signature length %d, got %d", clickurl.SignatureLength, len(sig))
	}
}

func TestVerify_Valid(t *testing.T) {
	signer := newTestSigner(t)
	message := "query1|result1|0|1|1700000000|https://example.com"

	sig := signer.Sign(message)

	if !signer.Verify(message, sig) {
		t.Fatal("expected valid signature to verify successfully")
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	signer := newTestSigner(t)
	message := "query1|result1|0|1|1700000000|https://example.com"

	invalidSig := "abcdef012345"

	if signer.Verify(message, invalidSig) {
		t.Fatal("expected random signature to fail verification")
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	signerA := clickurl.NewSigner("secret-a")
	signerB := clickurl.NewSigner("secret-b")

	message := "query1|result1|0|1|1700000000|https://example.com"
	sig := signerA.Sign(message)

	if signerB.Verify(message, sig) {
		t.Fatal("expected signature from different secret to fail verification")
	}
}

func TestSign_Deterministic(t *testing.T) {
	signer := newTestSigner(t)
	message := "deterministic-test-message"

	sig1 := signer.Sign(message)
	sig2 := signer.Sign(message)

	if sig1 != sig2 {
		t.Fatalf("expected identical signatures for same input, got %q and %q", sig1, sig2)
	}
}

func TestBuildMessage(t *testing.T) {
	params := clickurl.ClickParams{
		QueryID:        "q-123",
		ResultID:       "r-456",
		Position:       2,
		Page:           1,
		Timestamp:      1700000000,
		DestinationURL: "https://example.com/article",
	}

	expected := "q-123|r-456|2|1|1700000000|https://example.com/article"
	got := params.Message()

	if got != expected {
		t.Fatalf("expected message %q, got %q", expected, got)
	}
}
