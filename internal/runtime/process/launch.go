package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) startService(ctx context.Context, project domain.ProjectRuntime, service servicePlan) (*managedRun, bool, error) {
	runs, err := d.store.ListProjectRuns(ctx, project.ProjectID)
	if err != nil {
		return nil, false, err
	}
	for _, run := range runs {
		if run.ServiceID != service.service.ID || run.EndedAt != nil {
			continue
		}
		verified, verifyErr := d.verifiedMembersWithGrace(ctx, run, identityHandoffGrace)
		if verifyErr != nil {
			return nil, false, verifyErr
		}
		if len(verified) > 0 {
			return d.managedFor(run), false, nil
		}
		if err := d.store.FinishRun(ctx, run.ID, d.now().UTC(), nil, "identity_lost"); err != nil {
			return nil, false, err
		}
	}
	external, found, err := d.externalService(ctx, project, service)
	if err != nil {
		return nil, false, err
	}
	if found {
		return nil, false, fmt.Errorf("%w: %s is PID %d", ErrExternalProcess, service.service.ID, external.PID)
	}
	environment, err := d.resolveEnvironment(ctx, project.Process, service.definition)
	if err != nil {
		return nil, false, err
	}
	runID, err := identifier.New("run")
	if err != nil {
		return nil, false, err
	}
	buffer := newLogBuffer(defaultLogCapacity)
	managed := &managedRun{
		run: domain.RunRecord{
			ID: runID, ProjectID: project.ProjectID, ServiceID: service.service.ID,
			RuntimeDriver: domain.KindProcess, Origin: domain.OriginSwitchyard, OperationID: service.operationID,
		},
		project: project, service: service, logs: buffer,
	}
	command, identity, ownership, err := d.launch(ctx, managed, environment)
	if err != nil {
		return nil, false, err
	}
	managed.run.StartedAt = identity.StartedAt
	managed.run.IdentityFingerprint = identity.Fingerprint
	managed.run.Processes = []domain.ProcessIdentity{identity}
	managed.command = command
	managed.group = identity.ProcessGroup
	managed.ownership = ownership
	if err := d.store.CreateRun(ctx, managed.run); err != nil {
		abortOwnedProcess(command, ownership, identity.ProcessGroup)
		return nil, false, err
	}
	if err := d.store.RecordProcess(ctx, identity); err != nil {
		abortOwnedProcess(command, ownership, identity.ProcessGroup)
		_ = d.store.FinishRun(context.Background(), runID, d.now().UTC(), nil, "persistence_failed")
		return nil, false, err
	}
	d.mu.Lock()
	d.managed[serviceKey(project.ProjectID, service.service.ID)] = managed
	d.logs[runID] = buffer
	d.mu.Unlock()
	d.emit(project.ProjectID, domain.RuntimeEvent{
		Driver: domain.KindProcess, ProjectIdentity: project.ProjectSlug, ServiceIdentity: service.service.ID,
		RunID: runID, Action: "start", OccurredAt: d.now().UTC(),
	})
	go d.monitor(managed, command, environment)
	return managed, true, nil
}

func (d *Driver) launch(
	ctx context.Context,
	managed *managedRun,
	environment []string,
) (*exec.Cmd, domain.ProcessIdentity, processOwnership, error) {
	preview := previewCommand(managed.project.Root, managed.service.definition)
	command := exec.Command(preview.Executable, preview.Arguments...)
	command.Dir = preview.WorkingDirectory
	command.Env = environment
	configureProcessGroup(command)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, domain.ProcessIdentity{}, nil, err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return nil, domain.ProcessIdentity{}, nil, err
	}
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter
	if err := command.Start(); err != nil {
		closeProcessPipes(stdoutReader, stdoutWriter, stderrReader, stderrWriter)
		return nil, domain.ProcessIdentity{}, nil, fmt.Errorf("start %s: %w", managed.service.service.ID, err)
	}
	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	ownership, err := newProcessOwnership(command)
	if err != nil {
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
		abortOwnedProcess(command, nil, int32(command.Process.Pid))
		return nil, domain.ProcessIdentity{}, nil, fmt.Errorf("establish process-tree ownership: %w", err)
	}
	identity, err := d.snapshotStartedProcess(ctx, int32(command.Process.Pid))
	if err != nil {
		_ = stdoutReader.Close()
		_ = stderrReader.Close()
		abortOwnedProcess(command, ownership, int32(command.Process.Pid))
		return nil, domain.ProcessIdentity{}, nil, err
	}
	identity.ProcessGroup = ownership.Group()
	identity.RunID = managed.run.ID
	go d.captureLogs(stdoutReader, managed, "stdout")
	go d.captureLogs(stderrReader, managed, "stderr")
	return command, identity, ownership, nil
}

func (d *Driver) snapshotStartedProcess(ctx context.Context, pid int32) (domain.ProcessIdentity, error) {
	deadline := time.Now().Add(time.Second)
	var lastErr error
	for {
		identity, err := d.inspector.Snapshot(ctx, pid)
		if err == nil {
			return identity, nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return domain.ProcessIdentity{}, fmt.Errorf("inspect started PID %d: %w", pid, lastErr)
		}
		select {
		case <-ctx.Done():
			return domain.ProcessIdentity{}, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func closeProcessPipes(values ...*os.File) {
	for _, value := range values {
		_ = value.Close()
	}
}

func (d *Driver) captureLogs(reader io.ReadCloser, managed *managedRun, stream string) {
	defer func() { _ = reader.Close() }()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 256*1024)
	for scanner.Scan() {
		message := strings.TrimSuffix(scanner.Text(), "\r")
		managed.logs.add(domain.LogEntry{
			Timestamp: d.now().UTC(), ProjectID: managed.project.ProjectID, ServiceID: managed.service.service.ID,
			RunID: managed.run.ID, Source: "process", Stream: stream, Level: processLogLevel(message),
			Message: message, OperationID: managed.run.OperationID, Attributes: map[string]string{"process": managed.service.definition.ID},
		})
	}
}

func (d *Driver) managedFor(run domain.RunRecord) *managedRun {
	d.mu.RLock()
	managed := d.managed[serviceKey(run.ProjectID, run.ServiceID)]
	d.mu.RUnlock()
	if managed != nil {
		return managed
	}
	return &managedRun{run: run}
}

func (d *Driver) resolveEnvironment(ctx context.Context, config *domain.ProcessRuntime, definition domain.ProcessDefinition) ([]string, error) {
	values := environmentMap(os.Environ())
	references := make(map[string]domain.SecretReference)
	applyEnvironment(values, references, config.Environment, config.Secrets)
	applyEnvironment(values, references, definition.Environment, definition.Secrets)
	keys := make([]string, 0, len(references))
	for key := range references {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := d.secrets.Resolve(ctx, references[key])
		if err != nil {
			return nil, fmt.Errorf("resolve environment reference %s: %w", key, err)
		}
		values[key] = value
		if d.secretObserver != nil {
			d.secretObserver.AddSecret(value)
		}
	}
	keys = keys[:0]
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result, nil
}

func environmentMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func applyEnvironment(
	values map[string]string,
	references map[string]domain.SecretReference,
	overlay map[string]string,
	secrets map[string]domain.SecretReference,
) {
	for key, value := range overlay {
		values[key] = value
		delete(references, key)
	}
	for key, reference := range secrets {
		delete(values, key)
		references[key] = reference
	}
}

func processLogLevel(message string) string {
	for _, token := range strings.Fields(message) {
		switch strings.Trim(strings.ToLower(token), "[]:,") {
		case "trace", "debug", "info", "warn", "warning", "error", "fatal", "panic":
			return strings.Trim(strings.ToLower(token), "[]:,")
		}
		if len(token) > 12 {
			break
		}
	}
	return "unknown"
}
