//go:build darwin || linux

package process

import (
	"os/exec"
	"syscall"
)

type unixOwnership struct{ group int32 }

func newProcessOwnership(command *exec.Cmd) (processOwnership, error) {
	group, err := processGroupID(int32(command.Process.Pid))
	if err != nil {
		return nil, err
	}
	return unixOwnership{group: group}, nil
}

func (o unixOwnership) Group() int32 { return o.group }
func (o unixOwnership) Running() bool {
	err := syscall.Kill(-int(o.group), 0)
	return err == nil || err == syscall.EPERM
}
func (o unixOwnership) Signal(force bool) error { return signalProcessGroup(o.group, force) }
func (unixOwnership) Close() error              { return nil }

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

func isProcessGroupMember(_ int32, candidate, group int32) bool {
	candidateGroup, err := processGroupID(candidate)
	return err == nil && candidateGroup == group
}

func processGroupMatches(stored, current int32) bool { return stored == current }

func shellCommand(script string) (string, []string) {
	return "/bin/sh", []string{"-c", script}
}
