// Package correlation propagates request and operation correlation identifiers.
package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type contextKey struct{}

// NewID returns a cryptographically random, opaque correlation identifier.
func NewID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate correlation id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// WithID returns a context containing id.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// ID returns the correlation identifier, if present.
func ID(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
