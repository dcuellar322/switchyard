package adapters

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"switchyard.dev/switchyard/internal/discovery/application"
	"switchyard.dev/switchyard/internal/discovery/domain"
)

type composeScanner struct{}

func (composeScanner) Name() string { return "compose" }
func (composeScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		contents, err := root.ReadFile(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return scanCompose(name, contents)
	}
	for _, name := range []string{
		"compose.local.yaml", "compose.local.yml", "docker-compose.local.yaml", "docker-compose.local.yml",
		"compose.dev.yaml", "compose.dev.yml", "docker-compose.dev.yaml", "docker-compose.dev.yml",
		"compose.development.yaml", "compose.development.yml", "docker-compose.development.yaml", "docker-compose.development.yml",
	} {
		contents, err := root.ReadFile(name)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		items, err := scanCompose(name, contents)
		if err != nil {
			return nil, err
		}
		for index := range items {
			if items[index].Kind == "compose.project" {
				items[index].Warnings = append(items[index].Warnings, "using a nonstandard development Compose filename; review the selected file before approval")
			}
		}
		return items, nil
	}
	return nil, nil
}

type composeDocument struct {
	Name     string `yaml:"name"`
	Services map[string]struct {
		Ports    []yaml.Node `yaml:"ports"`
		Profiles []string    `yaml:"profiles"`
	} `yaml:"services"`
}

func scanCompose(path string, contents []byte) ([]domain.Evidence, error) {
	var document composeDocument
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	lines := strings.Split(string(contents), "\n")
	serviceNames := make([]string, 0, len(document.Services))
	profileSet := make(map[string]struct{})
	for name := range document.Services {
		serviceNames = append(serviceNames, name)
		for _, profile := range document.Services[name].Profiles {
			if profile != "" {
				profileSet[profile] = struct{}{}
			}
		}
	}
	sort.Strings(serviceNames)
	profiles := make([]string, 0, len(profileSet))
	for profile := range profileSet {
		profiles = append(profiles, profile)
	}
	sort.Strings(profiles)
	var result []domain.Evidence
	for _, name := range serviceNames {
		// Compose excludes profiled services unless that profile is explicitly
		// enabled. Discovery describes the default lifecycle, so optional
		// services must not make an otherwise healthy project look incomplete.
		if len(document.Services[name].Profiles) > 0 {
			continue
		}
		line := findYAMLKey(lines, name)
		items, err := evidence("compose.service", path, line, line, .98, map[string]any{"service": name})
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
		for index, portNode := range document.Services[name].Ports {
			portItems, err := composePortEvidence(path, name, index, portNode)
			if err != nil {
				return nil, err
			}
			result = append(result, portItems...)
		}
	}
	items, err := evidence("compose.project", path, 1, max(1, len(lines)), .99, map[string]any{"file": path, "projectName": document.Name, "profiles": profiles})
	return append(result, items...), err
}

func composePortEvidence(path, service string, index int, node yaml.Node) ([]domain.Evidence, error) {
	var raw any
	if err := node.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse %s service %s port: %w", path, service, err)
	}
	line := max(1, node.Line)
	host, target, protocol, ok := parseComposePort(raw)
	if ok {
		return evidence("compose.port", path, line, line, .96, map[string]any{
			"service": service, "host": host, "target": target, "protocol": protocol, "index": index,
		})
	}
	items, err := evidence("compose.port.unresolved", path, line, line, .5, map[string]any{"service": service, "index": index})
	if err == nil {
		items[0].Warnings = []string{"published Compose port has no deterministic numeric host value; set it in a local manifest overlay if it must be reserved"}
	}
	return items, err
}

func parseComposePort(raw any) (int, int, string, bool) {
	protocol := "tcp"
	switch value := raw.(type) {
	case string:
		value = strings.Trim(value, "\"'")
		if separator := strings.LastIndex(value, "/"); separator >= 0 {
			protocol = value[separator+1:]
			value = value[:separator]
		}
		mapping := splitComposePortMapping(value)
		if len(mapping) < 2 {
			return 0, 0, protocol, false
		}
		host, ok1 := composePortNumber(mapping[len(mapping)-2])
		target, ok2 := composePortNumber(mapping[len(mapping)-1])
		return host, target, protocol, ok1 && ok2
	case map[string]any:
		host, ok1 := composePortNumberValue(value["published"])
		target, ok2 := composePortNumberValue(value["target"])
		if candidate, ok := value["protocol"].(string); ok {
			protocol = candidate
		}
		return host, target, protocol, ok1 && ok2
	default:
		return 0, 0, protocol, false
	}
}

func composePortNumberValue(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, value >= 1 && value <= 65535
	case uint64:
		if value > uint64(math.MaxInt) {
			return 0, false
		}
		return int(value), value >= 1 && value <= 65535
	case string:
		return composePortNumber(value)
	default:
		return 0, false
	}
}

func splitComposePortMapping(value string) []string {
	var fields []string
	start, braces, brackets := 0, 0, 0
	for index, character := range value {
		switch character {
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case ':':
			if braces == 0 && brackets == 0 {
				fields = append(fields, value[start:index])
				start = index + 1
			}
		}
	}
	return append(fields, value[start:])
}

var composeDefaultPort = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*(?::-|-)([0-9]+)\}$`)

func composePortNumber(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if matches := composeDefaultPort.FindStringSubmatch(value); len(matches) == 2 {
		value = matches[1]
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil && parsed >= 1 && parsed <= 65535
}
