package process

import (
	"fmt"
	"math"
	"os/exec"
)

type processOwnership interface {
	Group() int32
	Running() bool
	Signal(bool) error
	Close() error
}

func boundedPID(pid int) (int32, error) {
	if pid <= 0 || int64(pid) > math.MaxInt32 {
		return 0, fmt.Errorf("process ID %d is outside the supported range", pid)
	}
	return int32(pid), nil
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
