package adapters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
	"switchyard.dev/switchyard/internal/plugins/domain"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

// TextRedactor removes credentials from captured plugin stderr.
type TextRedactor interface{ RedactText(string) (string, bool) }

// ProcessRunner starts one isolated process per bounded method call.
type ProcessRunner struct {
	hostVersion string
	redactor    TextRedactor
}

// NewProcessRunner creates a per-call process supervisor.
func NewProcessRunner(hostVersion string, redactor TextRedactor) *ProcessRunner {
	return &ProcessRunner{hostVersion: hostVersion, redactor: redactor}
}

// Call negotiates identity, invokes one method, and contains process failure.
func (r *ProcessRunner) Call(ctx context.Context, invocation pluginsApplication.Invocation) ([]domain.LogEntry, error) {
	callCtx, cancel := boundedCallContext(ctx)
	defer cancel()
	command := exec.CommandContext(callCtx, invocation.Plugin.Executable, invocation.Plugin.Arguments...)
	command.Dir = filepath.Dir(invocation.Plugin.ManifestPath)
	command.Env = minimalEnvironment()
	configurePluginProcess(command)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &boundedBuffer{limit: 64 << 10}
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start plugin %s: %w", invocation.Plugin.ID, err)
	}
	client := pluginsdk.NewClient(stdout, stdin)
	var initialized pluginsdk.InitializeResult
	initErr := client.Call("initialize", pluginsdk.InitializeParams{ProtocolVersion: pluginsdk.ProtocolVersion, HostVersion: r.hostVersion, GrantedScopes: invocation.Scopes}, &initialized)
	if initErr == nil {
		initErr = validateIdentity(invocation, initialized)
	}
	callErr := initErr
	if callErr == nil {
		callErr = client.Call(invocation.Method, invocation.Params, invocation.Result)
	}
	_ = stdin.Close()
	waitErr := waitPlugin(command)
	logs := r.logs(invocation.Plugin.ID, stderr.String())
	if callErr != nil {
		return logs, fmt.Errorf("plugin %s %s: %w", invocation.Plugin.ID, invocation.Method, callErr)
	}
	if waitErr != nil {
		return logs, fmt.Errorf("plugin %s exited unsuccessfully: %w", invocation.Plugin.ID, waitErr)
	}
	return logs, nil
}

func validateIdentity(invocation pluginsApplication.Invocation, initialized pluginsdk.InitializeResult) error {
	if err := initialized.Plugin.Validate(); err != nil {
		return fmt.Errorf("running plugin returned an invalid manifest: %w", err)
	}
	if initialized.ProtocolVersion != pluginsdk.ProtocolVersion || initialized.Plugin.ProtocolVersion != pluginsdk.ProtocolVersion ||
		initialized.Plugin.ID != invocation.Plugin.ID || initialized.Plugin.Name != invocation.Plugin.Name || initialized.Plugin.Version != invocation.Plugin.Version {
		return errors.New("running plugin identity does not match the reviewed manifest")
	}
	capabilities := slices.Clone(initialized.Plugin.Capabilities)
	slices.Sort(capabilities)
	expectedCapabilities := make([]pluginsdk.Capability, len(invocation.Plugin.Capabilities))
	for index, value := range invocation.Plugin.Capabilities {
		expectedCapabilities[index] = pluginsdk.Capability(value)
	}
	slices.Sort(expectedCapabilities)
	requested := slices.Clone(initialized.Plugin.RequestedScopes)
	slices.Sort(requested)
	expectedScopes := make([]pluginsdk.Scope, len(invocation.Plugin.RequestedScopes))
	for index, value := range invocation.Plugin.RequestedScopes {
		expectedScopes[index] = pluginsdk.Scope(value)
	}
	slices.Sort(expectedScopes)
	granted := slices.Clone(initialized.GrantedScopes)
	slices.Sort(granted)
	expectedGranted := slices.Clone(invocation.Scopes)
	slices.Sort(expectedGranted)
	if !slices.Equal(capabilities, expectedCapabilities) || !slices.Equal(requested, expectedScopes) || !slices.Equal(granted, expectedGranted) {
		return errors.New("running plugin capabilities or grants do not match the reviewed manifest")
	}
	return nil
}

func boundedCallContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, exists := ctx.Deadline(); exists {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, 15*time.Second)
}

func waitPlugin(command *exec.Cmd) error {
	const gracefulExitTimeout = 3 * time.Second

	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		terminatePluginProcess(command)
		return err
	case <-time.After(gracefulExitTimeout):
		terminatePluginProcess(command)
		return <-done
	}
}

func minimalEnvironment() []string {
	result := []string{"SWITCHYARD_PLUGIN_PROTOCOL=" + pluginsdk.ProtocolVersion}
	for _, name := range []string{"PATH", "TMPDIR", "LANG"} {
		if value, ok := os.LookupEnv(name); ok {
			result = append(result, name+"="+value)
		}
	}
	return result
}

func (r *ProcessRunner) logs(pluginID, value string) []domain.LogEntry {
	if r.redactor != nil {
		value, _ = r.redactor.RedactText(value)
	}
	lines := strings.Split(strings.ReplaceAll(value, "\r", ""), "\n")
	if len(lines) > 200 {
		lines = lines[len(lines)-200:]
	}
	result := []domain.LogEntry{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 2048 {
			line = line[:2048]
		}
		result = append(result, domain.LogEntry{PluginID: pluginID, Level: "info", Message: line, Created: time.Now().UTC()})
	}
	return result
}

type boundedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	original := len(value)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		_, _ = b.buffer.Write(value[:min(remaining, len(value))])
	}
	if original > remaining {
		b.truncated = true
	}
	return original, nil
}
func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	value := b.buffer.String()
	if b.truncated {
		value += "\n[plugin stderr truncated]"
	}
	return value
}

var _ io.Writer = (*boundedBuffer)(nil)
