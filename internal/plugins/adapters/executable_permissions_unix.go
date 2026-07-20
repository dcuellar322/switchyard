//go:build !windows

package adapters

import (
	"errors"
	"os"
)

func validateExecutablePermissions(mode os.FileMode) error {
	if mode.Perm()&0o111 == 0 {
		return errors.New("plugin file is not executable")
	}
	if mode.Perm()&0o022 != 0 {
		return errors.New("plugin executable cannot be group- or world-writable")
	}
	return nil
}
