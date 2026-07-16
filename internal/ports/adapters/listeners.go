package adapters

import (
	"bufio"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/ports/domain"
)

type commandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, executable string, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, executable, arguments...).Output()
}

// OSListeners reads local TCP and UDP listener facts through lsof's field format.
type OSListeners struct {
	runner commandRunner
	now    func() time.Time
}

// NewOSListeners creates an lsof-backed listener observer.
func NewOSListeners() *OSListeners { return &OSListeners{runner: execRunner{}, now: time.Now} }

// Facts returns deduplicated local TCP and UDP listener evidence.
func (s *OSListeners) Facts(ctx context.Context) ([]domain.Fact, error) {
	tcp, err := s.runner.Output(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-Fpcn")
	if err != nil && !emptyLsofResult(err, tcp) {
		return nil, fmt.Errorf("inspect TCP listeners with lsof: %w", err)
	}
	udp, err := s.runner.Output(ctx, "lsof", "-nP", "-iUDP", "-Fpcn")
	if err != nil && !emptyLsofResult(err, udp) {
		return nil, fmt.Errorf("inspect UDP listeners with lsof: %w", err)
	}
	return append(s.parse(tcp, "tcp"), s.parse(udp, "udp")...), nil
}

func emptyLsofResult(err error, output []byte) bool {
	var exitErr *exec.ExitError
	return len(output) == 0 && errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func (s *OSListeners) parse(output []byte, protocol string) []domain.Fact {
	var facts []domain.Fact
	seen := make(map[string]struct{})
	pid, command := 0, ""
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'c':
			command = line[1:]
		case 'n':
			host, port, ok := parseListenerName(line[1:])
			if !ok {
				continue
			}
			id := stableID("listener", protocol, host, port, pid)
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			facts = append(facts, domain.Fact{
				ID: id, Kind: domain.KindBinding,
				Host: host, Port: port, Protocol: protocol, Source: "os",
				Evidence: fmt.Sprintf("live listener owned by %s (pid %d)", command, pid), ProcessID: pid, ObservedAt: s.now().UTC(),
			})
		}
	}
	return facts
}

func parseListenerName(value string) (string, int, bool) {
	value = strings.TrimPrefix(value, "TCP ")
	value = strings.TrimPrefix(value, "UDP ")
	value = strings.SplitN(value, "->", 2)[0]
	separator := strings.LastIndex(value, ":")
	if separator < 0 {
		return "", 0, false
	}
	host := strings.Trim(value[:separator], "[]")
	port, err := strconv.Atoi(value[separator+1:])
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}
	if host == "*" {
		host = "0.0.0.0"
	}
	return host, port, true
}

func stableID(prefix string, parts ...any) string {
	digest := sha256.Sum256([]byte(fmt.Sprint(parts...)))
	return fmt.Sprintf("%s_%x", prefix, digest[:12])
}
