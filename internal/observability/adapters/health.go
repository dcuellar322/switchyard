// Package adapters implements local observability infrastructure boundaries.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

// HealthEvaluator evaluates trusted health declarations without a shell.
type HealthEvaluator struct {
	client *http.Client
	now    func() time.Time
}

// NewHealthEvaluator creates the production evaluator.
func NewHealthEvaluator() *HealthEvaluator {
	return &HealthEvaluator{client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now}
}

// Evaluate runs one bounded health probe.
func (e *HealthEvaluator) Evaluate(ctx context.Context, project runtime.ProjectRuntime, observation runtime.Observation, check runtime.HealthCheckDefinition) observability.HealthResult {
	started := e.now()
	status, message := e.evaluate(ctx, project, observation, check)
	return observability.HealthResult{Status: status, Message: message, LatencyMS: max(0, e.now().Sub(started).Milliseconds()), ObservedAt: e.now().UTC()}
}

func (e *HealthEvaluator) evaluate(ctx context.Context, project runtime.ProjectRuntime, observation runtime.Observation, check runtime.HealthCheckDefinition) (observability.HealthStatus, string) {
	switch check.Type {
	case "http":
		return e.http(ctx, expandPorts(check.URL, project.Ports), check)
	case "tcp":
		return tcp(ctx, expandPorts(check.Address, project.Ports))
	case "process":
		return observedService(observation, check.ServiceID, false)
	case "docker":
		return observedService(observation, check.ServiceID, true)
	case "command":
		return command(ctx, project.Root, check.Command)
	default:
		return observability.StatusUnknown, "unsupported health check type"
	}
}

func (e *HealthEvaluator) http(ctx context.Context, target string, check runtime.HealthCheckDefinition) (observability.HealthStatus, string) {
	parsedTarget, err := url.Parse(target)
	if err != nil || !loopbackHost(parsedTarget.Hostname()) {
		return observability.StatusUnhealthy, "HTTP check must target loopback"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return observability.StatusUnhealthy, "HTTP request is invalid"
	}
	response, err := e.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return observability.StatusUnhealthy, "HTTP check timed out"
		}
		return observability.StatusUnhealthy, "HTTP endpoint is unavailable"
	}
	defer func() { _ = response.Body.Close() }()
	expected := check.ExpectedStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	if response.StatusCode != expected {
		return observability.StatusUnhealthy, fmt.Sprintf("HTTP status %d, expected %d", response.StatusCode, expected)
	}
	if check.JSONPath == "" {
		return observability.StatusHealthy, fmt.Sprintf("HTTP status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return observability.StatusUnhealthy, "HTTP response could not be read"
	}
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return observability.StatusUnhealthy, "HTTP response is not JSON"
	}
	value, ok := jsonPath(document, check.JSONPath)
	if !ok || fmt.Sprint(value) != check.ExpectedValue {
		return observability.StatusUnhealthy, "HTTP JSON assertion did not match"
	}
	return observability.StatusHealthy, "HTTP JSON assertion passed"
}

func tcp(ctx context.Context, address string) (observability.HealthStatus, string) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || !loopbackHost(host) {
		return observability.StatusUnhealthy, "TCP check must target loopback"
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		if ctx.Err() != nil {
			return observability.StatusUnhealthy, "TCP check timed out"
		}
		return observability.StatusUnhealthy, "TCP endpoint is unavailable"
	}
	_ = connection.Close()
	return observability.StatusHealthy, "TCP connection succeeded"
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func observedService(observation runtime.Observation, serviceID string, requireDockerHealth bool) (observability.HealthStatus, string) {
	for _, service := range observation.Services {
		if service.ID != serviceID {
			continue
		}
		if requireDockerHealth {
			switch service.Health {
			case "healthy":
				return observability.StatusHealthy, "Docker health is healthy"
			case "unhealthy":
				return observability.StatusUnhealthy, "Docker health is unhealthy"
			default:
				return observability.StatusUnknown, "Docker health is not available"
			}
		}
		switch service.State {
		case "running", "paused":
			return observability.StatusHealthy, "process is alive"
		default:
			return observability.StatusUnhealthy, "process is not running"
		}
	}
	return observability.StatusUnknown, "service observation is unavailable"
}

func command(ctx context.Context, root string, arguments []string) (observability.HealthStatus, string) {
	if len(arguments) == 0 {
		return observability.StatusUnknown, "command is empty"
	}
	probe := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	probe.Dir = root
	if err := probe.Run(); err != nil {
		if ctx.Err() != nil {
			return observability.StatusUnhealthy, "command check timed out"
		}
		return observability.StatusUnhealthy, "command exited unsuccessfully"
	}
	return observability.StatusHealthy, "command exited successfully"
}

func jsonPath(document any, path string) (any, bool) {
	path = strings.TrimPrefix(strings.TrimSpace(path), "$")
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return document, true
	}
	current := document
	for _, part := range strings.Split(path, ".") {
		if index, err := strconv.Atoi(part); err == nil {
			values, ok := current.([]any)
			if !ok || index < 0 || index >= len(values) {
				return nil, false
			}
			current = values[index]
			continue
		}
		values, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = values[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func expandPorts(value string, ports map[string]runtime.PortDeclaration) string {
	for id, port := range ports {
		value = strings.ReplaceAll(value, "${ports."+id+"}", strconv.Itoa(port.Host))
	}
	return value
}
