package process

import (
	"context"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) streamMetrics(ctx context.Context, request domain.MetricRequest, sink domain.MetricSink) error {
	runs, err := d.store.ListProjectRuns(ctx, request.Project.ProjectID)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	for _, run := range runs {
		if run.EndedAt != nil || (request.Service != "" && run.ServiceID != request.Service) {
			continue
		}
		if _, exists := seen[run.ServiceID]; exists {
			continue
		}
		verified, err := verifiedRunMembers(ctx, d.inspector, run)
		if err != nil || len(verified) == 0 {
			continue
		}
		identity, _ := primaryIdentity(verified)
		usage, err := d.inspector.Usage(ctx, identity.PID)
		if err != nil {
			return err
		}
		if err := sink.WriteMetric(ctx, domain.MetricSample{
			Timestamp: d.now().UTC(), ProjectID: run.ProjectID, ServiceID: run.ServiceID,
			CPUPercent: usage.cpuPercent, MemoryBytes: usage.memoryBytes, MemoryLimit: usage.memoryLimit,
		}); err != nil {
			return err
		}
		seen[run.ServiceID] = struct{}{}
	}
	return nil
}
