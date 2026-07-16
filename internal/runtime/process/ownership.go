package process

import "os/exec"

type processOwnership interface {
	Group() int32
	Running() bool
	Signal(bool) error
	Close() error
}

func closeOwnership(ownership processOwnership) {
	if ownership != nil {
		_ = ownership.Close()
	}
}

func abortOwnedProcess(command *exec.Cmd, ownership processOwnership, fallbackGroup int32) {
	if ownership != nil {
		_ = ownership.Signal(true)
		_ = ownership.Close()
	} else {
		_ = signalProcessGroup(fallbackGroup, true)
	}
	_, _ = command.Process.Wait()
}
