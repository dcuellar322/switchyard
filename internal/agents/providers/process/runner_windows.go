//go:build windows

package process

import "os/exec"

func configureProcessGroup(*exec.Cmd) {}
func terminateProcessGroup(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
