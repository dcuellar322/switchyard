//go:build windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	gprocess "github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

type windowsOwnership struct {
	mu     sync.Mutex
	job    windows.Handle
	group  int32
	closed bool
}

type jobBasicAccountingInformation struct {
	totalUserTime             int64
	totalKernelTime           int64
	thisPeriodTotalUserTime   int64
	thisPeriodTotalKernelTime int64
	totalPageFaultCount       uint32
	totalProcesses            uint32
	activeProcesses           uint32
	totalTerminatedProcesses  uint32
}

type jobBasicProcessIDListHeader struct {
	numberOfAssignedProcesses uint32
	numberOfProcessIDsInList  uint32
}

func configureProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

func newProcessOwnership(command *exec.Cmd, pid int32) (processOwnership, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	err = command.Process.WithHandle(func(handle uintptr) {
		err = windows.AssignProcessToJobObject(job, windows.Handle(handle))
	})
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &windowsOwnership{job: job, group: pid}, nil
}

func (o *windowsOwnership) Group() int32 { return o.group }

func (o *windowsOwnership) Running() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return false
	}
	info := jobBasicAccountingInformation{}
	err := windows.QueryInformationJobObject(
		o.job,
		windows.JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	)
	return err == nil && info.activeProcesses > 0
}

func (o *windowsOwnership) MemberPIDs() ([]int32, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil, os.ErrProcessDone
	}

	capacity := 8
	headerSize := unsafe.Sizeof(jobBasicProcessIDListHeader{})
	pidSize := unsafe.Sizeof(uintptr(0))
	headerWords := (int(headerSize) + int(pidSize) - 1) / int(pidSize)
	for {
		buffer := make([]uintptr, headerWords+capacity)
		header := (*jobBasicProcessIDListHeader)(unsafe.Pointer(&buffer[0]))
		// #nosec G115 -- the buffer is bounded above to a few megabytes.
		bufferBytes := uint32(len(buffer) * int(pidSize))
		err := windows.QueryInformationJobObject(
			o.job,
			windows.JobObjectBasicProcessIdList,
			uintptr(unsafe.Pointer(&buffer[0])),
			bufferBytes,
			nil,
		)
		incomplete := header.numberOfProcessIDsInList < header.numberOfAssignedProcesses
		if err == windows.ERROR_MORE_DATA || (err == nil && incomplete) {
			if capacity >= 1<<20 || header.numberOfAssignedProcesses > 1<<20 {
				return nil, fmt.Errorf(
					"job object membership exceeded supported capacity: %d assigned processes",
					header.numberOfAssignedProcesses,
				)
			}
			capacity *= 2
			continue
		}
		if err != nil {
			return nil, err
		}
		if header.numberOfProcessIDsInList > uint32(capacity) {
			return nil, fmt.Errorf(
				"job object returned %d process IDs for capacity %d",
				header.numberOfProcessIDsInList,
				capacity,
			)
		}
		nativePIDs := buffer[headerWords : headerWords+int(header.numberOfProcessIDsInList)]
		pids := make([]int32, 0, len(nativePIDs))
		for _, pid := range nativePIDs {
			if pid == 0 || pid > uintptr(^uint32(0)>>1) {
				return nil, fmt.Errorf("job object returned unsupported process ID %d", pid)
			}
			// #nosec G115 -- the native PID was bounded to signed 32-bit above.
			pids = append(pids, int32(pid))
		}
		return pids, nil
	}
}

func (o *windowsOwnership) Signal(force bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return os.ErrProcessDone
	}
	if force {
		return windows.TerminateJobObject(o.job, 1)
	}
	// CTRL_BREAK_EVENT is the graceful signal available to a new Windows
	// process group. A console-less host may reject it; the bounded stop path
	// then escalates through the Job Object without losing child ownership.
	_ = windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(o.group))
	return nil
}

func (o *windowsOwnership) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil
	}
	o.closed = true
	return windows.CloseHandle(o.job)
}

func processGroupID(pid int32) (int32, error) { return pid, nil }

func signalProcessGroup(group int32, _ bool) error {
	process, err := os.FindProcess(int(group))
	if err != nil {
		return err
	}
	return process.Kill()
}

func isProcessGroupMember(parent, candidate, group int32) bool {
	if candidate == group {
		return true
	}
	for depth := 0; depth < 64 && parent > 0; depth++ {
		if parent == group {
			return true
		}
		process, err := gprocess.NewProcess(parent)
		if err != nil {
			return false
		}
		parent, err = process.Ppid()
		if err != nil {
			return false
		}
	}
	return false
}

// Windows process identities retain the owning Job Object's synthetic group.
// A child may be reparented after its launcher exits, so the kernel-reported
// parent chain is not a durable group identifier.
func processGroupMatches(_, _ int32) bool { return true }

func shellCommand(script string) (string, []string) {
	return "cmd.exe", []string{"/D", "/S", "/C", script}
}
