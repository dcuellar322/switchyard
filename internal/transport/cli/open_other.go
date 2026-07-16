//go:build !darwin && !linux && !windows

package cli

import "errors"

func openURL(string) error {
	return errors.New("opening URLs is unsupported on this platform; use --print")
}
