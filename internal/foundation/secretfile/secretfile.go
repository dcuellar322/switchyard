// Package secretfile validates filesystem protections before private key use.
package secretfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Validate rejects missing, non-regular, or insufficiently protected secret files.
func Validate(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve private key: %w", err)
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return fmt.Errorf("inspect private key: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("private key must resolve to a regular file")
	}
	return validatePermissions(info.Mode())
}
