package process

import (
	"context"
	"errors"
	"fmt"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type keychainResolver struct{}

func (keychainResolver) Resolve(ctx context.Context, reference domain.SecretReference) (string, error) {
	if reference.Provider != "keychain" || reference.Key == "" {
		return "", errors.New("invalid keychain reference")
	}
	value, err := resolveKeychain(ctx, reference)
	if err != nil {
		return "", fmt.Errorf("resolve keychain item %q: %w", reference.Key, err)
	}
	return value, nil
}
