//go:build !darwin && !linux && !windows

package process

import (
	"errors"
	"os/exec"
)

type unsupportedOwnership struct{ group int32 }

func newProcessOwnership(_ *exec.Cmd, pid int32) (processOwnership, error) {
	return unsupportedOwnership{group: pid}, nil
}

func (o unsupportedOwnership) Group() int32            { return o.group }
func (unsupportedOwnership) Running() bool             { return false }
func (o unsupportedOwnership) Signal(force bool) error { return signalProcessGroup(o.group, force) }
func (unsupportedOwnership) Close() error              { return nil }

func configureProcessGroup(_ *exec.Cmd) {}

func processGroupID(pid int32) (int32, error) { return pid, nil }

func signalProcessGroup(int32, bool) error {
	return errors.New("process groups are unsupported on this platform")
}

func isProcessGroupMember(_ int32, candidate, group int32) bool { return candidate == group }

func processGroupMatches(stored, current int32) bool { return stored == current }

func shellCommand(script string) (string, []string) { return "/bin/sh", []string{"-c", script} }
