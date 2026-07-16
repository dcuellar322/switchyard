// Command archcheck enforces Switchyard's package dependency policy.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const modulePath = "switchyard.dev/switchyard"

func main() {
	packages, err := listPackages(context.Background())
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	violations := analyze(modulePath, packages)
	frontendViolations, err := analyzeFrontend("web/src")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	violations = append(violations, frontendViolations...)
	sortViolations(violations)
	for _, item := range violations {
		_, _ = fmt.Fprintln(os.Stderr, item)
	}
	if len(violations) > 0 {
		os.Exit(1)
	}
}

func listPackages(ctx context.Context) ([]packageInfo, error) {
	command := exec.CommandContext(ctx, "go", "list", "-json", "-deps", "./...")
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("go list packages: %s", exitErr.Stderr)
		}
		return nil, fmt.Errorf("go list packages: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	var packages []packageInfo
	for {
		var item packageInfo
		err := decoder.Decode(&item)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		packages = append(packages, item)
	}
	return packages, nil
}
