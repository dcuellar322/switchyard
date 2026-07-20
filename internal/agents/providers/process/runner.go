// Package process provides bounded subprocess execution for local AI provider CLIs.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"switchyard.dev/switchyard/internal/platform/processgroup"
)

// Command is an argument-array subprocess request with a deliberately small environment.
type Command struct {
	Executable  string
	Args        []string
	Directory   string
	Stdin       []byte
	Environment []string
	OutputLimit int64
}

// Result carries bounded process streams.
type Result struct{ Stdout, Stderr []byte }

// Runner is the provider subprocess test seam.
type Runner interface {
	Run(context.Context, Command) (Result, error)
}

// OSRunner executes provider CLIs without a shell.
type OSRunner struct{}

// Run executes one command and kills its process group when the context ends.
func (OSRunner) Run(ctx context.Context, request Command) (Result, error) {
	if !filepath.IsAbs(request.Executable) || request.Directory == "" || request.OutputLimit < 1 {
		return Result{}, errors.New("invalid provider process request")
	}
	command := exec.CommandContext(ctx, request.Executable, request.Args...)
	command.Dir = request.Directory
	command.Env = append([]string(nil), request.Environment...)
	command.Stdin = bytes.NewReader(request.Stdin)
	processgroup.Configure(command)
	stdout := &limitedBuffer{limit: request.OutputLimit}
	stderr := &limitedBuffer{limit: request.OutputLimit}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Start(); err != nil {
		return Result{}, err
	}
	ownership, err := processgroup.Own(command)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return Result{}, fmt.Errorf("contain provider process: %w", err)
	}
	defer func() { _ = ownership.Close() }()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		_ = ownership.Terminate()
		result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
		if stdout.exceeded || stderr.exceeded {
			return result, fmt.Errorf("provider process output exceeded %d bytes", request.OutputLimit)
		}
		return result, err
	case <-ctx.Done():
		_ = ownership.Terminate()
		<-done
		return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, ctx.Err()
	}
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int64
	exceeded bool
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	remaining := b.limit - int64(b.Len())
	if remaining <= 0 {
		b.exceeded = true
		return original, nil
	}
	if int64(len(value)) > remaining {
		value = value[:remaining]
		b.exceeded = true
	}
	_, _ = b.Buffer.Write(value)
	return original, nil
}

// CanonicalExecutable resolves only absolute PATH entries, preventing repository-relative shadowing.
func CanonicalExecutable(configured string) (string, error) {
	if configured == "" {
		return "", errors.New("provider executable is required")
	}
	if filepath.IsAbs(configured) {
		return executableFile(configured)
	}
	if strings.ContainsRune(configured, filepath.Separator) {
		return "", errors.New("provider executable must be an absolute path or bare command name")
	}
	for _, directory := range filepath.SplitList(os.Getenv("PATH")) {
		if !filepath.IsAbs(directory) {
			continue
		}
		if resolved, err := executableFile(filepath.Join(directory, configured)); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("provider executable %q not found on an absolute PATH entry", configured)
}

func executableFile(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("provider executable %q is not executable", path)
	}
	return resolved, nil
}

// AllowEnvironment returns only explicitly named variables and never the entire daemon environment.
func AllowEnvironment(names ...string) []string {
	result := []string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			result = append(result, name+"="+value)
		}
	}
	return result
}

func readBounded(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	value, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(value)) > limit {
		return nil, fmt.Errorf("provider result exceeded %d bytes", limit)
	}
	return value, nil
}

// ReadBounded reads a provider result file with a hard cap.
func ReadBounded(path string, limit int64) ([]byte, error) { return readBounded(path, limit) }
