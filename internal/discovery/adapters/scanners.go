// Package adapters implements bounded deterministic repository scanners.
package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	"switchyard.dev/switchyard/internal/discovery/application"
	"switchyard.dev/switchyard/internal/discovery/domain"
	manifest "switchyard.dev/switchyard/internal/manifest/application"
)

// Defaults returns every Phase 3 deterministic scanner in stable order.
func Defaults() []application.Scanner {
	return []application.Scanner{gitScanner{}, composeScanner{}, pythonScanner{}, nodeScanner{}, makeScanner{}, justScanner{}, readmeScanner{}, existingRuntimeScanner{}}
}

type gitScanner struct{}

func (gitScanner) Name() string { return "git" }
func (gitScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	contents, err := root.ReadFile(".git/HEAD")
	if errors.Is(err, syscall.ENOTDIR) {
		contents, fileErr := root.ReadFile(".git")
		if fileErr != nil {
			return nil, fileErr
		}
		if !strings.HasPrefix(strings.TrimSpace(string(contents)), "gitdir: ") {
			return nil, errors.New("unsupported .git file")
		}
		return evidence("git.repository", ".git", 1, 1, .95, map[string]any{"worktree": true})
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	branch := strings.TrimSpace(string(contents))
	branch = strings.TrimPrefix(branch, "ref: refs/heads/")
	return evidence("git.repository", ".git/HEAD", 1, 1, 1, map[string]any{"defaultBranch": branch})
}

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
	return nil, nil
}

type composeDocument struct {
	Name     string `yaml:"name"`
	Services map[string]struct {
		Command  any      `yaml:"command"`
		Ports    []any    `yaml:"ports"`
		Profiles []string `yaml:"profiles"`
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
		for index, raw := range document.Services[name].Ports {
			host, target, protocol, ok := parseComposePort(raw)
			if !ok {
				continue
			}
			portLine := findAfter(lines, line, strconv.Itoa(target))
			items, err = evidence("compose.port", path, portLine, portLine, .96, map[string]any{
				"service": name, "host": host, "target": target, "protocol": protocol, "index": index,
			})
			if err != nil {
				return nil, err
			}
			result = append(result, items...)
		}
	}
	items, err := evidence("compose.project", path, 1, max(1, len(lines)), .99, map[string]any{"file": path, "projectName": document.Name, "profiles": profiles})
	return append(result, items...), err
}

func parseComposePort(raw any) (int, int, string, bool) {
	protocol := "tcp"
	switch value := raw.(type) {
	case string:
		value = strings.Trim(value, "\"'")
		parts := strings.Split(value, "/")
		if len(parts) == 2 {
			protocol = parts[1]
		}
		mapping := strings.Split(parts[0], ":")
		if len(mapping) < 2 {
			return 0, 0, protocol, false
		}
		host, err1 := strconv.Atoi(mapping[len(mapping)-2])
		target, err2 := strconv.Atoi(mapping[len(mapping)-1])
		return host, target, protocol, err1 == nil && err2 == nil
	case map[string]any:
		host, ok1 := number(value["published"])
		target, ok2 := number(value["target"])
		if candidate, ok := value["protocol"].(string); ok {
			protocol = candidate
		}
		return host, target, protocol, ok1 && ok2
	default:
		return 0, 0, protocol, false
	}
}

func number(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, true
	case uint64:
		return int(value), true
	case string:
		parsed, err := strconv.Atoi(value)
		return parsed, err == nil
	default:
		return 0, false
	}
}

type pythonScanner struct{}

func (pythonScanner) Name() string { return "python" }
func (pythonScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	contents, err := root.ReadFile("pyproject.toml")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		Project struct {
			Name string `toml:"name"`
		} `toml:"project"`
	}
	if err := toml.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse pyproject.toml: %w", err)
	}
	manager := "python"
	if _, err := root.ReadFile("uv.lock"); err == nil {
		manager = "uv"
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	line := findTextLine(contents, "name")
	return evidence("python.project", "pyproject.toml", line, line, .92, map[string]any{
		"name": document.Project.Name, "manager": manager, "testCommand": []string{"uv", "run", "pytest"},
	})
}

type nodeScanner struct{}

func (nodeScanner) Name() string { return "node" }
func (nodeScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	contents, err := root.ReadFile("package.json")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		Name, PackageManager string
		Scripts              map[string]string
	}
	if err := json.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	manager := strings.Split(document.PackageManager, "@")[0]
	if manager == "" {
		manager = "npm"
	}
	var result []domain.Evidence
	for _, script := range []string{"dev", "start", "test", "build", "lint"} {
		if _, ok := document.Scripts[script]; !ok {
			continue
		}
		line := findTextLine(contents, `"`+script+`"`)
		items, err := evidence("node.script", "package.json", line, line, .9, map[string]any{
			"name": document.Name, "script": script, "command": []string{manager, "run", script},
		})
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
	}
	return result, nil
}

var targetPattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.-]*):(?:\s|$)`)

type makeScanner struct{}

func (makeScanner) Name() string { return "make" }
func (makeScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	return scanTargets(root, "Makefile", "make.target", targetPattern)
}

var justPattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_-]*)(?:\s[^:]+)?:\s*$`)

type justScanner struct{}

func (justScanner) Name() string { return "just" }
func (justScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	return scanTargets(root, "justfile", "just.target", justPattern)
}

func scanTargets(root application.Root, path, kind string, pattern *regexp.Regexp) ([]domain.Evidence, error) {
	contents, err := root.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []domain.Evidence
	scanner := bufio.NewScanner(strings.NewReader(string(contents)))
	for line := 1; scanner.Scan(); line++ {
		matches := pattern.FindStringSubmatch(scanner.Text())
		if len(matches) != 2 || strings.HasPrefix(matches[1], ".") {
			continue
		}
		command := []string{strings.Split(kind, ".")[0], matches[1]}
		items, err := evidence(kind, path, line, line, .88, map[string]any{"target": matches[1], "command": command})
		if err != nil {
			return nil, err
		}
		result = append(result, items...)
	}
	return result, scanner.Err()
}

type readmeScanner struct{}

func (readmeScanner) Name() string { return "readme" }
func (readmeScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	for _, path := range []string{"README.md", "README", "README.txt"} {
		contents, err := root.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for index, line := range strings.Split(string(contents), "\n") {
			if strings.HasPrefix(line, "# ") {
				title, redacted := redactText(strings.TrimSpace(strings.TrimPrefix(line, "# ")))
				items, evidenceErr := evidence("readme.title", path, index+1, index+1, .7, map[string]any{"title": title})
				if redacted && len(items) > 0 {
					items[0].Warnings = []string{"suspected credential was redacted from README title"}
				}
				return items, evidenceErr
			}
		}
	}
	return nil, nil
}

type existingRuntimeScanner struct{}

func (existingRuntimeScanner) Name() string { return "existing-runtime" }
func (existingRuntimeScanner) Scan(_ context.Context, root application.Root) ([]domain.Evidence, error) {
	const path = ".switchyard/project.yml"
	contents, err := root.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	document, err := manifest.ParseYAML(contents)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return evidence("switchyard.manifest", path, 1, max(1, len(strings.Split(string(contents), "\n"))), 1, document)
}

func evidence(kind, path string, start, end int, confidence float64, data any) ([]domain.Evidence, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return []domain.Evidence{{Kind: kind, SourcePath: filepath.ToSlash(path), Location: domain.SourceRange{StartLine: start, EndLine: end}, Confidence: confidence, Data: encoded}}, nil
}

func findYAMLKey(lines []string, key string) int { return findAfter(lines, 1, key+":") }
func findAfter(lines []string, from int, value string) int {
	for index := max(0, from-1); index < len(lines); index++ {
		if strings.Contains(lines[index], value) {
			return index + 1
		}
	}
	return max(1, from)
}
func findTextLine(contents []byte, value string) int {
	return findAfter(strings.Split(string(contents), "\n"), 1, value)
}

var suspectedSecret = regexp.MustCompile(`(?i)(?:sk-[a-z0-9_-]{8,}|(?:token|password|secret)\s*[:=]\s*\S+)`)

func redactText(value string) (string, bool) {
	redacted := suspectedSecret.ReplaceAllString(value, "[redacted]")
	return redacted, redacted != value
}
