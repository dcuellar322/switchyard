package process

import (
	"context"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) stopAll(ctx context.Context, plan executionPlan, sink domain.ProgressSink, reason string) error {
	runs, err := d.store.ListProjectRuns(ctx, plan.project.ProjectID)
	if err != nil {
		return err
	}
	active := make(map[string]domain.RunRecord)
	for _, run := range runs {
		if run.EndedAt == nil {
			active[run.ServiceID] = run
		}
	}
	for _, service := range plan.services {
		run, ok := active[service.service.ID]
		if !ok {
			if err := sink.Step(ctx, "process.stop."+service.service.ID, "succeeded", "already stopped"); err != nil {
				return err
			}
			continue
		}
		timeout := defaultStopTimeout
		if service.definition.StopTimeoutSeconds > 0 {
			timeout = time.Duration(service.definition.StopTimeoutSeconds) * time.Second
		}
		if err := d.stopRun(ctx, run, timeout, reason); err != nil {
			return err
		}
		if err := sink.Step(ctx, "process.stop."+service.service.ID, "succeeded", "process group stopped"); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) stopRun(ctx context.Context, run domain.RunRecord, timeout time.Duration, reason string) error {
	verified, err := d.verifiedMembersWithGrace(ctx, run, identityHandoffGrace)
	if err != nil {
		return err
	}
	if len(verified) == 0 {
		return d.store.FinishRun(ctx, run.ID, d.now().UTC(), nil, "identity_lost")
	}
	d.mu.RLock()
	managed := d.managed[serviceKey(run.ProjectID, run.ServiceID)]
	d.mu.RUnlock()
	if managed != nil {
		managed.mu.Lock()
		managed.stopping = true
		managed.mu.Unlock()
	}
	groups := uniqueGroups(verified)
	for _, group := range groups {
		if err := signalProcessGroup(group, false); err != nil && d.groupStillRunning(ctx, group) {
			return fmt.Errorf("gracefully stop process group %d: %w", group, err)
		}
	}
	forced, err := d.waitForGroups(ctx, groups, timeout)
	if err != nil {
		return err
	}
	if forced {
		reason += "_forced"
	}
	endedAt := d.now().UTC()
	if err := d.store.FinishRun(context.WithoutCancel(ctx), run.ID, endedAt, nil, reason); err != nil {
		return err
	}
	if managed != nil {
		managed.mu.Lock()
		managed.run.EndedAt = &endedAt
		managed.run.TerminationReason = reason
		managed.mu.Unlock()
	}
	d.mu.Lock()
	delete(d.managed, serviceKey(run.ProjectID, run.ServiceID))
	d.mu.Unlock()
	d.emit(run.ProjectID, domain.RuntimeEvent{
		Driver: domain.KindProcess, ServiceIdentity: run.ServiceID, RunID: run.ID,
		Action: reason, OccurredAt: endedAt,
	})
	return nil
}

func (d *Driver) verifiedMembersWithGrace(
	ctx context.Context,
	run domain.RunRecord,
	grace time.Duration,
) ([]domain.ProcessIdentity, error) {
	deadline := time.Now().Add(grace)
	for {
		verified, err := verifiedRunMembers(ctx, d.inspector, run)
		if err != nil || len(verified) > 0 {
			return verified, err
		}
		if time.Now().After(deadline) {
			return nil, nil
		}
		runs, err := d.store.ListProjectRuns(ctx, run.ProjectID)
		if err != nil {
			return nil, err
		}
		for _, current := range runs {
			if current.ID == run.ID {
				run = current
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func (d *Driver) waitForGroups(ctx context.Context, groups []int32, timeout time.Duration) (bool, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !d.anyGroupRunning(ctx, groups) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			for _, group := range groups {
				if d.groupStillRunning(context.Background(), group) {
					_ = signalProcessGroup(group, true)
				}
			}
			killCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			for d.anyGroupRunning(killCtx, groups) {
				select {
				case <-killCtx.Done():
					return true, fmt.Errorf("process group did not terminate after escalation: %w", killCtx.Err())
				case <-time.After(25 * time.Millisecond):
				}
			}
			return true, nil
		case <-ticker.C:
		}
	}
}

func (d *Driver) anyGroupRunning(ctx context.Context, groups []int32) bool {
	for _, group := range groups {
		if d.groupStillRunning(ctx, group) {
			return true
		}
	}
	return false
}

func uniqueGroups(identities []domain.ProcessIdentity) []int32 {
	seen := make(map[int32]struct{})
	result := []int32{}
	for _, identity := range identities {
		if _, ok := seen[identity.ProcessGroup]; ok {
			continue
		}
		seen[identity.ProcessGroup] = struct{}{}
		result = append(result, identity.ProcessGroup)
	}
	return result
}
