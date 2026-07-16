package adapters

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"syscall"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
	gprocess "github.com/shirou/gopsutil/v4/process"

	"switchyard.dev/switchyard/internal/ports/domain"
)

type connectionReader func(context.Context, string) ([]gnet.ConnectionStat, error)
type processNameReader func(context.Context, int32) string

// OSListeners reads local TCP and UDP bindings through native OS APIs exposed
// by gopsutil. It does not depend on lsof, ss, netstat, or PowerShell output.
type OSListeners struct {
	connections connectionReader
	processName processNameReader
	now         func() time.Time
}

// NewOSListeners creates the portable listener observer.
func NewOSListeners() *OSListeners {
	return &OSListeners{connections: gnet.ConnectionsWithContext, processName: observedProcessName, now: time.Now}
}

// Facts returns deduplicated local TCP listeners and UDP bindings.
func (s *OSListeners) Facts(ctx context.Context) ([]domain.Fact, error) {
	connections, err := s.connections(ctx, "inet")
	if err != nil {
		return nil, fmt.Errorf("inspect operating-system listeners: %w", err)
	}
	observedAt := s.now().UTC()
	seen := make(map[string]struct{})
	result := make([]domain.Fact, 0, len(connections))
	for _, connection := range connections {
		protocol, ok := listenerProtocol(connection)
		if !ok || connection.Laddr.Port == 0 {
			continue
		}
		host := normalizeListenerHost(connection.Laddr.IP)
		pid := int(connection.Pid)
		id := stableID("listener", protocol, host, connection.Laddr.Port, pid)
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		name := "unknown process"
		if pid > 0 && s.processName != nil {
			if observed := s.processName(ctx, connection.Pid); observed != "" {
				name = observed
			}
		}
		result = append(result, domain.Fact{
			ID: id, Kind: domain.KindBinding, Host: host, Port: int(connection.Laddr.Port), Protocol: protocol,
			Source: "os", Evidence: fmt.Sprintf("live listener owned by %s (pid %d)", name, pid),
			ProcessID: pid, ObservedAt: observedAt,
		})
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Port != result[right].Port {
			return result[left].Port < result[right].Port
		}
		if result[left].Protocol != result[right].Protocol {
			return result[left].Protocol < result[right].Protocol
		}
		return result[left].ID < result[right].ID
	})
	return result, nil
}

func listenerProtocol(connection gnet.ConnectionStat) (string, bool) {
	switch connection.Type {
	case syscall.SOCK_STREAM:
		return "tcp", strings.EqualFold(connection.Status, "LISTEN")
	case syscall.SOCK_DGRAM:
		return "udp", true
	default:
		return "", false
	}
}

func normalizeListenerHost(host string) string {
	if host == "" || host == "*" || host == "::" {
		return "0.0.0.0"
	}
	return strings.Trim(host, "[]")
}

func observedProcessName(ctx context.Context, pid int32) string {
	process, err := gprocess.NewProcessWithContext(ctx, pid)
	if err != nil {
		return ""
	}
	name, _ := process.NameWithContext(ctx)
	return name
}

func stableID(prefix string, parts ...any) string {
	digest := sha256.Sum256([]byte(fmt.Sprint(parts...)))
	return fmt.Sprintf("%s_%x", prefix, digest[:12])
}
