//go:build darwin || linux

package process

import (
	"os/exec"
	"syscall"
)

func configureProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processGroupID(pid int32) (int32, error) {
	group, err := syscall.Getpgid(int(pid))
	return int32(group), err
}

func signalProcessGroup(group int32, force bool) error {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	return syscall.Kill(-int(group), signal)
}

func shellCommand(script string) (string, []string) {
	return "/bin/sh", []string{"-c", script}
}
