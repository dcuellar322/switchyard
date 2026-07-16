//go:build windows

package cli

import "os/exec"

func openURL(target string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
}
