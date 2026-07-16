//go:build darwin

package cli

import "os/exec"

func openURL(target string) error { return exec.Command("open", target).Start() }
