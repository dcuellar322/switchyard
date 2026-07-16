package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	gprocess "github.com/shirou/gopsutil/v4/process"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

type processUsage struct {
	cpuPercent  float64
	memoryBytes uint64
	memoryLimit uint64
}

type processInspector interface {
	Snapshot(context.Context, int32) (domain.ProcessIdentity, error)
	GroupMembers(context.Context, int32) ([]domain.ProcessIdentity, error)
	Listeners(context.Context, int) ([]domain.ProcessIdentity, error)
	MatchesCommand(context.Context, int32, string) bool
	Usage(context.Context, int32) (processUsage, error)
}

type gopsutilInspector struct{}

func (gopsutilInspector) Snapshot(ctx context.Context, pid int32) (domain.ProcessIdentity, error) {
	process, err := gprocess.NewProcessWithContext(ctx, pid)
	if err != nil {
		return domain.ProcessIdentity{}, err
	}
	createdMillis, err := process.CreateTimeWithContext(ctx)
	if err != nil {
		return domain.ProcessIdentity{}, err
	}
	executable, err := process.ExeWithContext(ctx)
	if err != nil || executable == "" {
		arguments, argumentsErr := process.CmdlineSliceWithContext(ctx)
		if argumentsErr != nil || len(arguments) == 0 {
			return domain.ProcessIdentity{}, errors.Join(err, argumentsErr)
		}
		executable = arguments[0]
	}
	workingDirectory, err := process.CwdWithContext(ctx)
	if err != nil {
		return domain.ProcessIdentity{}, err
	}
	group, err := processGroupID(pid)
	if err != nil {
		return domain.ProcessIdentity{}, err
	}
	startedAt := time.UnixMilli(createdMillis).UTC()
	executable = canonicalPath(executable)
	workingDirectory = canonicalPath(workingDirectory)
	return domain.ProcessIdentity{
		PID: pid, ProcessGroup: group, Executable: executable, StartedAt: startedAt,
		WorkingDirectory: workingDirectory, Fingerprint: processFingerprint(executable, startedAt, workingDirectory),
		ObservedAt: time.Now().UTC(),
	}, nil
}

func (inspector gopsutilInspector) GroupMembers(ctx context.Context, group int32) ([]domain.ProcessIdentity, error) {
	processes, err := gprocess.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	result := []domain.ProcessIdentity{}
	for _, process := range processes {
		candidateGroup, groupErr := processGroupID(process.Pid)
		if groupErr != nil || candidateGroup != group {
			continue
		}
		identity, snapshotErr := inspector.Snapshot(ctx, process.Pid)
		if snapshotErr == nil {
			result = append(result, identity)
		}
	}
	return result, nil
}

func (inspector gopsutilInspector) Listeners(ctx context.Context, port int) ([]domain.ProcessIdentity, error) {
	connections, err := gnet.ConnectionsWithContext(ctx, "tcp")
	if err != nil {
		return nil, err
	}
	seen := make(map[int32]struct{})
	result := []domain.ProcessIdentity{}
	for _, connection := range connections {
		if connection.Status != "LISTEN" || int(connection.Laddr.Port) != port || connection.Pid <= 0 {
			continue
		}
		if _, ok := seen[connection.Pid]; ok {
			continue
		}
		seen[connection.Pid] = struct{}{}
		identity, snapshotErr := inspector.Snapshot(ctx, connection.Pid)
		if snapshotErr == nil {
			result = append(result, identity)
		}
	}
	return result, nil
}

func (gopsutilInspector) MatchesCommand(ctx context.Context, pid int32, expected string) bool {
	expected = strings.ToLower(strings.TrimSuffix(filepath.Base(expected), ".exe"))
	for depth := 0; depth < 5 && pid > 0; depth++ {
		process, err := gprocess.NewProcessWithContext(ctx, pid)
		if err != nil {
			return false
		}
		executable, _ := process.ExeWithContext(ctx)
		arguments, _ := process.CmdlineSliceWithContext(ctx)
		if commandTokenMatches(expected, executable) {
			return true
		}
		for _, argument := range arguments {
			if commandTokenMatches(expected, argument) {
				return true
			}
		}
		parent, err := process.ParentWithContext(ctx)
		if err != nil || parent == nil {
			return false
		}
		pid = parent.Pid
	}
	return false
}

func commandTokenMatches(expected, value string) bool {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(value), ".exe"))
	if base == expected {
		return true
	}
	return strings.Contains(base, expected+"-cli.") || strings.HasPrefix(base, expected+".")
}

func (gopsutilInspector) Usage(ctx context.Context, pid int32) (processUsage, error) {
	process, err := gprocess.NewProcessWithContext(ctx, pid)
	if err != nil {
		return processUsage{}, err
	}
	cpuPercent, cpuErr := process.CPUPercentWithContext(ctx)
	memory, memoryErr := process.MemoryInfoWithContext(ctx)
	hostMemory, hostErr := mem.VirtualMemoryWithContext(ctx)
	if err := errors.Join(cpuErr, memoryErr, hostErr); err != nil {
		return processUsage{}, err
	}
	return processUsage{cpuPercent: max(0, cpuPercent), memoryBytes: memory.RSS, memoryLimit: hostMemory.Total}, nil
}

func processFingerprint(executable string, startedAt time.Time, workingDirectory string) string {
	digest := sha256.Sum256([]byte(executable + "\x00" + startedAt.UTC().Format(time.RFC3339Nano) + "\x00" + workingDirectory))
	return hex.EncodeToString(digest[:])
}

func canonicalPath(value string) string {
	if value == "" {
		return ""
	}
	absolute, err := filepath.Abs(value)
	if err == nil {
		value = absolute
	}
	resolved, err := filepath.EvalSymlinks(value)
	if err == nil {
		value = resolved
	}
	return filepath.Clean(value)
}

func identityMatches(stored, current domain.ProcessIdentity) bool {
	return stored.PID == current.PID && stored.ProcessGroup == current.ProcessGroup &&
		stored.Fingerprint != "" && stored.Fingerprint == current.Fingerprint
}

func executableMatches(command string, identity domain.ProcessIdentity) bool {
	if command == "" || identity.Executable == "" {
		return false
	}
	commandBase := strings.ToLower(strings.TrimSuffix(filepath.Base(command), ".exe"))
	identityBase := strings.ToLower(strings.TrimSuffix(filepath.Base(identity.Executable), ".exe"))
	return commandBase == identityBase
}

func verifiedRunMembers(ctx context.Context, inspector processInspector, run domain.RunRecord) ([]domain.ProcessIdentity, error) {
	verified := []domain.ProcessIdentity{}
	for _, stored := range run.Processes {
		current, err := inspector.Snapshot(ctx, stored.PID)
		if err != nil {
			continue
		}
		if identityMatches(stored, current) {
			current.RunID = run.ID
			verified = append(verified, current)
		}
	}
	return verified, nil
}

func primaryIdentity(values []domain.ProcessIdentity) (domain.ProcessIdentity, error) {
	if len(values) == 0 {
		return domain.ProcessIdentity{}, fmt.Errorf("no verified process identity")
	}
	result := values[0]
	for _, value := range values[1:] {
		if value.StartedAt.Before(result.StartedAt) {
			result = value
		}
	}
	return result, nil
}
