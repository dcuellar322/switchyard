//go:build !windows

package adapters

import "os"

func replaceFile(source, destination string) error {
	return os.Rename(source, destination)
}
