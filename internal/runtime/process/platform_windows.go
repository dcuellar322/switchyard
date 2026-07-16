//go:build windows

package process

import (
	"os"
	"os/exec"
)

func configureProcessGroup(_ *exec.Cmd) {}

func processGroupID(pid int32) (int32, error) { return pid, nil }

func signalProcessGroup(group int32, _ bool) error {
	process, err := os.FindProcess(int(group))
	if err != nil {
		return err
	}
	return process.Kill()
}

func shellCommand(script string) (string, []string) {
	return "cmd.exe", []string{"/D", "/S", "/C", script}
}
