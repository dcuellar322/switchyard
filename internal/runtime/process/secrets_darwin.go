//go:build darwin

package process

import (
	"context"
	"os/exec"
	"strings"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func resolveKeychain(ctx context.Context, reference domain.SecretReference) (string, error) {
	arguments := []string{"find-generic-password", "-w", "-s", reference.Key}
	if reference.Account != "" {
		arguments = append(arguments, "-a", reference.Account)
	}
	output, err := exec.CommandContext(ctx, "/usr/bin/security", arguments...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}
