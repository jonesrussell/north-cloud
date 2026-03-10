package leadership

import (
	"crypto/sha256"
	"encoding/hex"
)

// ContentHash returns a hex-encoded SHA-256 hash of the given content.
// Used for change detection on leadership/contact pages.
func ContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
