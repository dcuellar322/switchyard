//go:build !windows

package adapters

import "os"

// commitExclusive publishes a same-directory temporary file without ever
// replacing a destination created after the caller's preview check.
func commitExclusive(source, destination string) error {
	if err := os.Link(source, destination); err != nil {
		return err
	}
	_ = os.Remove(source)
	return nil
}
