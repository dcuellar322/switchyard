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
	"slices"
	"sort"
	"strings"
	"syscall"

	toml "github.com/pelletier/go-toml/v2"

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
			Name    string            `toml:"name"`
			Scripts map[string]string `toml:"scripts"`
		} `toml:"project"`
	}
	if err := toml.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse pyproject.toml: %w", err)
	}
	manager := "python"
	if exists, existsErr := root.HasFile("uv.lock"); existsErr != nil {
		return nil, existsErr
	} else if exists {
		manager = "uv"
	}
	line := findTextLine(contents, "name")
	result, err := evidence("python.project", "pyproject.toml", line, line, .92, map[string]any{
		"name": document.Project.Name, "manager": manager, "testCommand": pythonCommand(manager, "pytest"),
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(document.Project.Scripts))
	for name := range document.Project.Scripts {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		items, evidenceErr := evidence("python.script", "pyproject.toml", findTextLine(contents, name+" ="), findTextLine(contents, name+" ="), .9, map[string]any{
			"name": name, "command": pythonCommand(manager, name), "preferredRun": len(keys) == 1 || slices.Contains([]string{"dev", "serve", "start"}, name),
		})
		if evidenceErr != nil {
			return nil, evidenceErr
		}
		result = append(result, items...)
	}
	return result, nil
}

func pythonCommand(manager, command string) []string {
	if manager == "uv" {
		return []string{"uv", "run", command}
	}
	return []string{command}
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
	manager, warnings, err := nodePackageManager(root, document.PackageManager)
	if err != nil {
		return nil, err
	}
	var result []domain.Evidence
	for _, script := range []string{"dev", "start", "test", "build", "lint"} {
		if _, ok := document.Scripts[script]; !ok {
			continue
		}
		line := findTextLine(contents, `"`+script+`"`)
		items, err := evidence("node.script", "package.json", line, line, .9, map[string]any{
			"name": document.Name, "manager": manager, "script": script, "command": []string{manager, "run", script},
		})
		if err != nil {
			return nil, err
		}
		items[0].Warnings = append(items[0].Warnings, warnings...)
		result = append(result, items...)
	}
	return result, nil
}

func nodePackageManager(root application.Root, declared string) (string, []string, error) {
	if declared != "" {
		manager := strings.SplitN(declared, "@", 2)[0]
		if slices.Contains([]string{"npm", "pnpm", "yarn", "bun"}, manager) {
			return manager, nil, nil
		}
		return "npm", []string{"unrecognized packageManager value; defaulting reviewed commands to npm"}, nil
	}
	for _, candidate := range []struct{ path, manager string }{
		{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"bun.lock", "bun"}, {"bun.lockb", "bun"}, {"package-lock.json", "npm"},
	} {
		exists, err := root.HasFile(candidate.path)
		if err != nil {
			return "", nil, err
		}
		if exists {
			return candidate.manager, nil, nil
		}
	}
	return "npm", nil, nil
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
		var result []domain.Evidence
		for index, line := range strings.Split(string(contents), "\n") {
			if strings.HasPrefix(line, "# ") {
				title, redacted := redactText(strings.TrimSpace(strings.TrimPrefix(line, "# ")))
				items, evidenceErr := evidence("readme.title", path, index+1, index+1, .7, map[string]any{"title": title})
				if evidenceErr != nil {
					return nil, evidenceErr
				}
				if redacted && len(items) > 0 {
					items[0].Warnings = []string{"suspected credential was redacted from README title"}
				}
				result = append(result, items...)
				break
			}
		}
		commands, err := scanDocumentedCommands(path, contents)
		if err != nil {
			return nil, err
		}
		return append(result, commands...), nil
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
