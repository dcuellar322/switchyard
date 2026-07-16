//go:build !darwin && !linux

package process

import (
	"context"
	"errors"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func resolveKeychain(context.Context, domain.SecretReference) (string, error) {
	return "", errors.New("operating-system keychain lookup is unavailable")
}
