package process

import (
	"context"
	"os/exec"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) monitor(managed *managedRun, command *exec.Cmd, environment []string) {
	trackingDone := make(chan struct{})
	go d.trackGroup(managed, trackingDone)
	err := command.Wait()
	close(trackingDone)
	exitCode := command.ProcessState.ExitCode()
	d.recordGroupMembers(context.Background(), managed)
	managed.mu.Lock()
	stopping := managed.stopping
	current := managed.command == command
	restartCount := managed.run.RestartCount
	managed.mu.Unlock()
	if stopping || !current {
		return
	}
	if d.managedGroupStillRunning(context.Background(), managed, managed.group) {
		d.waitForOrphanedGroup(managed, exitCode)
		return
	}
	if exitCode != 0 && managed.service.definition.Restart.Mode == "on-failure" &&
		restartCount < managed.service.definition.Restart.MaxRetries && d.ctx.Err() == nil {
		d.restartAfterCrash(managed, environment, exitCode)
		return
	}
	reason := "exited"
	if err != nil || exitCode != 0 {
		reason = "crashed"
	}
	d.finishManaged(managed, &exitCode, reason)
}

func (d *Driver) restartAfterCrash(managed *managedRun, environment []string, priorExit int) {
	backoff := time.Duration(managed.service.definition.Restart.BackoffSeconds) * time.Second
	select {
	case <-d.ctx.Done():
		d.finishManaged(managed, &priorExit, "crashed")
		return
	case <-time.After(backoff):
	}
	managed.mu.Lock()
	if managed.stopping {
		managed.mu.Unlock()
		return
	}
	managed.run.RestartCount++
	restartCount := managed.run.RestartCount
	managed.mu.Unlock()
	if err := d.store.SetRestartCount(context.Background(), managed.run.ID, restartCount); err != nil {
		d.finishManaged(managed, &priorExit, "restart_persistence_failed")
		return
	}
	managed.mu.Lock()
	priorOwnership := managed.ownership
	managed.mu.Unlock()
	closeOwnership(priorOwnership)
	command, identity, ownership, err := d.launch(d.ctx, managed, environment)
	if err != nil {
		d.finishManaged(managed, &priorExit, "restart_failed")
		return
	}
	if err := d.store.RecordProcess(context.Background(), identity); err != nil {
		abortOwnedProcess(command, ownership, identity.ProcessGroup)
		d.finishManaged(managed, &priorExit, "restart_persistence_failed")
		return
	}
	managed.mu.Lock()
	managed.command = command
	managed.group = identity.ProcessGroup
	managed.ownership = ownership
	managed.run.Processes = append(managed.run.Processes, identity)
	managed.mu.Unlock()
	d.emit(managed.project.ProjectID, domain.RuntimeEvent{
		Driver: domain.KindProcess, ProjectIdentity: managed.project.ProjectSlug,
		ServiceIdentity: managed.service.service.ID, RunID: managed.run.ID,
		Action: "restart_after_crash", OccurredAt: d.now().UTC(),
	})
	go d.monitor(managed, command, environment)
}

func (d *Driver) waitForOrphanedGroup(managed *managedRun, exitCode int) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		managed.mu.Lock()
		stopping := managed.stopping
		managed.mu.Unlock()
		if stopping {
			return
		}
		if !d.managedGroupStillRunning(context.Background(), managed, managed.group) {
			d.finishManaged(managed, &exitCode, "process_tree_exited")
			return
		}
		d.recordGroupMembers(context.Background(), managed)
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Driver) trackGroup(managed *managedRun, done <-chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.recordGroupMembers(context.Background(), managed)
		}
	}
}

func (d *Driver) recordGroupMembers(ctx context.Context, managed *managedRun) {
	managed.mu.Lock()
	group := managed.group
	runID := managed.run.ID
	managed.mu.Unlock()
	members, err := d.inspector.GroupMembers(ctx, group)
	if err != nil {
		return
	}
	for _, member := range members {
		member.RunID = runID
		if d.store.RecordProcess(ctx, member) == nil {
			managed.mu.Lock()
			if !containsIdentity(managed.run.Processes, member) {
				managed.run.Processes = append(managed.run.Processes, member)
			}
			managed.mu.Unlock()
		}
	}
}

func containsIdentity(values []domain.ProcessIdentity, candidate domain.ProcessIdentity) bool {
	for _, value := range values {
		if value.PID == candidate.PID && value.StartedAt.Equal(candidate.StartedAt) {
			return true
		}
	}
	return false
}

func (d *Driver) groupStillRunning(ctx context.Context, group int32) bool {
	members, err := d.inspector.GroupMembers(ctx, group)
	return err == nil && len(members) > 0
}

func (d *Driver) managedGroupStillRunning(ctx context.Context, managed *managedRun, group int32) bool {
	if managed != nil {
		managed.mu.Lock()
		ownership := managed.ownership
		managedGroup := managed.group
		managed.mu.Unlock()
		if ownership != nil && managedGroup == group {
			return ownership.Running()
		}
	}
	return d.groupStillRunning(ctx, group)
}

func (d *Driver) finishManaged(managed *managedRun, exitCode *int, reason string) {
	managed.mu.Lock()
	if managed.run.EndedAt != nil {
		managed.mu.Unlock()
		return
	}
	endedAt := d.now().UTC()
	managed.run.EndedAt = &endedAt
	managed.run.ExitCode = exitCode
	managed.run.TerminationReason = reason
	ownership := managed.ownership
	managed.ownership = nil
	managed.mu.Unlock()
	closeOwnership(ownership)
	_ = d.store.FinishRun(context.Background(), managed.run.ID, endedAt, exitCode, reason)
	d.mu.Lock()
	delete(d.managed, serviceKey(managed.project.ProjectID, managed.service.service.ID))
	d.mu.Unlock()
	d.emit(managed.project.ProjectID, domain.RuntimeEvent{
		Driver: domain.KindProcess, ProjectIdentity: managed.project.ProjectSlug,
		ServiceIdentity: managed.service.service.ID, RunID: managed.run.ID,
		Action: reason, OccurredAt: endedAt,
	})
}
