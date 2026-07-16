//go:build linux

package process

import (
	"context"
	"os/exec"
	"strings"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func resolveKeychain(ctx context.Context, reference domain.SecretReference) (string, error) {
	arguments := []string{"lookup", "service", reference.Key}
	if reference.Account != "" {
		arguments = append(arguments, "account", reference.Account)
	}
	output, err := exec.CommandContext(ctx, "secret-tool", arguments...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}
