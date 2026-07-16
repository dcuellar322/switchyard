//go:build windows

package adapters

import "os/exec"

func configurePluginProcess(*exec.Cmd) {}

func terminatePluginProcess(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
