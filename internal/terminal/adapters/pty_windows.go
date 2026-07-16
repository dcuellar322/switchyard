//go:build windows

package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

// WindowsPTY starts daemon-owned interactive sessions through ConPTY.
type WindowsPTY struct{}

// NewPTY returns the native Windows ConPTY adapter.
func NewPTY() *WindowsPTY { return &WindowsPTY{} }

// Start creates a pseudo console and starts the reviewed argument-array command.
func (*WindowsPTY) Start(ctx context.Context, plan application.LaunchPlan, size domain.Size) (application.Process, error) {
	process, err := startConPTY(plan, size)
	if err != nil {
		return nil, err
	}
	go func() {
		select {
		case <-ctx.Done():
			terminateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = process.Terminate(terminateCtx)
			cancel()
		case <-process.done:
		}
	}()
	return process, nil
}

type windowsPTYProcess struct {
	input   *os.File
	output  *os.File
	console windows.Handle
	process windows.Handle
	job     windows.Handle
	pid     int
	done    chan struct{}
	waitErr error
	close   sync.Once
}

type windowsExitError struct{ code int }

func (e windowsExitError) Error() string { return fmt.Sprintf("process exited with code %d", e.code) }
func (e windowsExitError) ExitCode() int { return e.code }

func startConPTY(plan application.LaunchPlan, size domain.Size) (*windowsPTYProcess, error) {
	input, output, console, err := createPseudoConsole(size)
	if err != nil {
		return nil, err
	}
	processHandle, threadHandle, pid, err := createPseudoConsoleProcess(console, plan)
	if err != nil {
		_ = input.Close()
		_ = output.Close()
		windows.ClosePseudoConsole(console)
		return nil, err
	}
	_ = windows.CloseHandle(threadHandle)
	job, err := createTerminalJob(processHandle)
	if err != nil {
		_ = windows.TerminateProcess(processHandle, 1)
		_ = windows.CloseHandle(processHandle)
		_ = input.Close()
		_ = output.Close()
		windows.ClosePseudoConsole(console)
		return nil, fmt.Errorf("assign ConPTY process to Job Object: %w", err)
	}
	process := &windowsPTYProcess{
		input: input, output: output, console: console, process: processHandle, job: job, pid: int(pid), done: make(chan struct{}),
	}
	go process.waitForExit()
	return process, nil
}

func createPseudoConsole(size domain.Size) (*os.File, *os.File, windows.Handle, error) {
	var inputRead, inputWrite, outputRead, outputWrite windows.Handle
	if err := windows.CreatePipe(&inputRead, &inputWrite, nil, 0); err != nil {
		return nil, nil, 0, fmt.Errorf("create ConPTY input pipe: %w", err)
	}
	if err := windows.CreatePipe(&outputRead, &outputWrite, nil, 0); err != nil {
		_ = windows.CloseHandle(inputRead)
		_ = windows.CloseHandle(inputWrite)
		return nil, nil, 0, fmt.Errorf("create ConPTY output pipe: %w", err)
	}
	var console windows.Handle
	err := windows.CreatePseudoConsole(windows.Coord{X: int16(size.Columns), Y: int16(size.Rows)}, inputRead, outputWrite, 0, &console)
	_ = windows.CloseHandle(inputRead)
	_ = windows.CloseHandle(outputWrite)
	if err != nil {
		_ = windows.CloseHandle(inputWrite)
		_ = windows.CloseHandle(outputRead)
		return nil, nil, 0, fmt.Errorf("create Windows pseudo console: %w", err)
	}
	return os.NewFile(uintptr(inputWrite), "conpty-input"), os.NewFile(uintptr(outputRead), "conpty-output"), console, nil
}

func createPseudoConsoleProcess(
	console windows.Handle,
	plan application.LaunchPlan,
) (windows.Handle, windows.Handle, uint32, error) {
	executable, err := exec.LookPath(plan.Executable)
	if err != nil {
		return 0, 0, 0, err
	}
	executablePointer, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return 0, 0, 0, err
	}
	arguments := windows.ComposeCommandLine(append([]string{executable}, plan.Arguments...))
	argumentPointer, err := windows.UTF16PtrFromString(arguments)
	if err != nil {
		return 0, 0, 0, err
	}
	directoryPointer, err := windows.UTF16PtrFromString(plan.WorkingDirectory)
	if err != nil {
		return 0, 0, 0, err
	}
	environment, err := windowsEnvironmentBlock(terminalEnvironment(plan.Environment))
	if err != nil {
		return 0, 0, 0, err
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return 0, 0, 0, err
	}
	defer attributes.Delete()
	if err := attributes.Update(
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		unsafe.Pointer(&console),
		unsafe.Sizeof(console),
	); err != nil {
		return 0, 0, 0, err
	}
	startup := windows.StartupInfoEx{
		StartupInfo:             windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{}))},
		ProcThreadAttributeList: attributes.List(),
	}
	information := windows.ProcessInformation{}
	err = windows.CreateProcess(
		executablePointer, argumentPointer, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_NEW_PROCESS_GROUP,
		&environment[0], directoryPointer, &startup.StartupInfo, &information,
	)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("start ConPTY command: %w", err)
	}
	return information.Process, information.Thread, information.ProcessId, nil
}

func windowsEnvironmentBlock(values []string) ([]uint16, error) {
	result := make([]uint16, 0, 4096)
	for _, value := range values {
		if strings.IndexByte(value, 0) >= 0 {
			return nil, errors.New("terminal environment contains a NUL byte")
		}
		encoded, err := windows.UTF16FromString(value)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded...)
	}
	result = append(result, 0)
	return result, nil
}

func createTerminalJob(process windows.Handle) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(
		job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)),
	)
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
	}
	if err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func (p *windowsPTYProcess) waitForExit() {
	_, err := windows.WaitForSingleObject(p.process, windows.INFINITE)
	if err == nil {
		var exitCode uint32
		err = windows.GetExitCodeProcess(p.process, &exitCode)
		if err == nil && exitCode != 0 {
			err = windowsExitError{code: int(exitCode)}
		}
	}
	p.waitErr = err
	close(p.done)
}

func (p *windowsPTYProcess) Read(value []byte) (int, error)  { return p.output.Read(value) }
func (p *windowsPTYProcess) Write(value []byte) (int, error) { return p.input.Write(value) }

func (p *windowsPTYProcess) Resize(size domain.Size) error {
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(size.Columns), Y: int16(size.Rows)})
}

func (p *windowsPTYProcess) Terminate(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	_, _ = p.input.Write([]byte{3})
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		_ = windows.TerminateJobObject(p.job, 1)
		return ctx.Err()
	case <-timer.C:
		return windows.TerminateJobObject(p.job, 1)
	}
}

func (p *windowsPTYProcess) Wait() error { <-p.done; return p.waitErr }
func (p *windowsPTYProcess) PID() int    { return p.pid }

func (p *windowsPTYProcess) Close() error {
	var result error
	p.close.Do(func() {
		result = errors.Join(p.input.Close(), p.output.Close())
		windows.ClosePseudoConsole(p.console)
		result = errors.Join(result, windows.CloseHandle(p.process), windows.CloseHandle(p.job))
	})
	return result
}

var _ io.ReadWriter = (*windowsPTYProcess)(nil)
