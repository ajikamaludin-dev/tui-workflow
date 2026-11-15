package editor

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashContent returns the SHA-256 hex digest of b. Used as the content
// baseline for the external file watcher so writes that preserve mtime are
// still detected.
func HashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
