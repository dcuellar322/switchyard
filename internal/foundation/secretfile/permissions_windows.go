//go:build windows

package secretfile

import "os"

// Windows access is governed by ACLs rather than portable mode bits. The file
// type check still rejects directories, devices, and unresolved special files.
func validatePermissions(os.FileMode) error { return nil }
