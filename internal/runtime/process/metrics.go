package process

import (
	"context"
	"sort"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) streamMetrics(ctx context.Context, request domain.MetricRequest, sink domain.MetricSink) error {
	runs, err := d.store.ListProjectRuns(ctx, request.Project.ProjectID)
	if err != nil {
		return err
	}
	samples := map[string]domain.MetricSample{}
	for _, run := range runs {
		if run.EndedAt != nil || (request.Service != "" && run.ServiceID != request.Service) {
			continue
		}
		verified, err := verifiedRunMembers(ctx, d.inspector, run)
		if err != nil || len(verified) == 0 {
			continue
		}
		sample := samples[run.ServiceID]
		sample.Timestamp, sample.ProjectID, sample.ServiceID = d.now().UTC(), run.ProjectID, run.ServiceID
		sample.InstanceID = run.ID
		sample.RestartCount += run.RestartCount
		for _, identity := range verified {
			usage, usageErr := d.inspector.Usage(ctx, identity.PID)
			if usageErr != nil {
				sample.Partial = true
				continue
			}
			sample.CPUPercent += usage.cpuPercent
			sample.CPUAvailable = sample.CPUAvailable || usage.cpuAvailable
			sample.MemoryBytes += usage.memoryBytes
			sample.MemoryLimit = max(sample.MemoryLimit, usage.memoryLimit)
			sample.MemoryAvailable = sample.MemoryAvailable || usage.memoryAvailable
			sample.DiskReadBytes += usage.diskReadBytes
			sample.DiskWriteBytes += usage.diskWriteBytes
			sample.DiskAvailable = sample.DiskAvailable || usage.diskAvailable
			sample.ProcessCount++
		}
		if sample.ProcessCount > 0 {
			samples[run.ServiceID] = sample
		}
	}
	serviceIDs := make([]string, 0, len(samples))
	for serviceID := range samples {
		serviceIDs = append(serviceIDs, serviceID)
	}
	sort.Strings(serviceIDs)
	for _, serviceID := range serviceIDs {
		if err := sink.WriteMetric(ctx, samples[serviceID]); err != nil {
			return err
		}
	}
	return nil
}
