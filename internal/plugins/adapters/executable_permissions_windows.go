//go:build windows

package adapters

import "os"

// Windows executable access is governed by ACLs and the PE loader rather than
// portable Unix mode bits. The shared caller still rejects symlinks and every
// non-regular file before reaching this platform boundary.
func validateExecutablePermissions(os.FileMode) error { return nil }
