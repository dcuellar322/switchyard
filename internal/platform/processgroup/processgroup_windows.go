//go:build windows

// Package processgroup contains one-shot subprocess trees across platforms.
package processgroup

import (
	"errors"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Ownership retains the Job Object created for one command.
type Ownership struct{ job windows.Handle }

// Configure prepares a command to own a new Windows process group.
func Configure(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
}

// Own assigns a started command to a kill-on-close Job Object.
func Own(command *exec.Cmd) (*Ownership, error) {
	if command.Process == nil {
		return nil, errors.New("process has not started")
	}
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
	if err == nil {
		var assignErr error
		err = command.Process.WithHandle(func(handle uintptr) {
			assignErr = windows.AssignProcessToJobObject(job, windows.Handle(handle))
		})
		if err == nil {
			err = assignErr
		}
	}
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &Ownership{job: job}, nil
}

// Terminate kills every process retained by the Job Object.
func (ownership *Ownership) Terminate() error {
	if ownership == nil || ownership.job == 0 {
		return nil
	}
	return windows.TerminateJobObject(ownership.job, 1)
}

// Close releases the Job Object and its kernel resources.
func (ownership *Ownership) Close() error {
	if ownership == nil || ownership.job == 0 {
		return nil
	}
	job := ownership.job
	ownership.job = 0
	return windows.CloseHandle(job)
}
