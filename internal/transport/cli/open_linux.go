//go:build linux

package cli

import "os/exec"

func openURL(target string) error { return exec.Command("xdg-open", target).Start() }
