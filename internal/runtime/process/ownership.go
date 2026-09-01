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

// processMemberSource exposes kernel-owned membership when the platform can
// provide it directly. Windows Job Objects implement this boundary; recovered
// runs without a live ownership handle fall back to inspector discovery.
type processMemberSource interface {
	MemberPIDs() ([]int32, error)
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
