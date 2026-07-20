//go:build !windows

package secretfile

import (
	"errors"
	"os"
)

func validatePermissions(mode os.FileMode) error {
	if mode.Perm()&0o077 != 0 {
		return errors.New("private key must not be accessible by group or other users")
	}
	return nil
}
