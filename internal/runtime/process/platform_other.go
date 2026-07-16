//go:build !darwin && !linux && !windows

package process

import (
	"errors"
	"os/exec"
)

func configureProcessGroup(_ *exec.Cmd) {}

func processGroupID(pid int32) (int32, error) { return pid, nil }

func signalProcessGroup(int32, bool) error {
	return errors.New("process groups are unsupported on this platform")
}

func shellCommand(script string) (string, []string) { return "/bin/sh", []string{"-c", script} }
