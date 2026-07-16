// Package identifier creates opaque local entity identifiers.
package identifier

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a cryptographically random identifier with a readable prefix.
func New(prefix string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate %s identifier: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}
