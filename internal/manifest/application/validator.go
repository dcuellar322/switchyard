package application

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"switchyard.dev/switchyard/internal/manifest/domain"
	manifestSchema "switchyard.dev/switchyard/internal/manifest/schema"
)

// ValidationResult separates blocking errors from portability warnings.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// Validate runs schema, domain, containment, executable, port, and health checks.
func Validate(root string, manifest domain.Manifest) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []string{}, Warnings: []string{}}
	if err := validateSchema(manifest); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if err := manifest.Validate(); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	for _, path := range manifestPaths(manifest) {
		if err := containedPath(root, path); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}
	for _, action := range manifest.Actions {
		if action.Type == "command" && len(action.Command) > 0 {
			if _, err := exec.LookPath(action.Command[0]); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("action %q executable %q is unavailable", action.ID, action.Command[0]))
			}
			if action.Shell {
				result.Warnings = append(result.Warnings, fmt.Sprintf("action %q opts into shell interpretation", action.ID))
			}
		}
	}
	for _, check := range healthChecks(manifest.Services) {
		if check.Type == "http" && !loopbackURL(check.URL) {
			result.Errors = append(result.Errors, "HTTP health checks must target loopback by default")
		}
		if check.Type == "tcp" && !loopbackAddress(check.Address) {
			result.Errors = append(result.Errors, "TCP health checks must target loopback by default")
		}
		if check.Type == "command" && len(check.Command) > 0 {
			if _, err := exec.LookPath(check.Command[0]); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("health check executable %q is unavailable", check.Command[0]))
			}
		}
	}
	result.Valid = len(result.Errors) == 0
	return result
}

func loopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loopbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && loopbackHost(parsed.Hostname())
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateSchema(manifest domain.Manifest) error {
	compiler := jsonschema.NewCompiler()
	var schemaDocument any
	if err := json.Unmarshal(manifestSchema.Project, &schemaDocument); err != nil {
		return fmt.Errorf("decode manifest schema: %w", err)
	}
	if err := compiler.AddResource("project.schema.json", schemaDocument); err != nil {
		return fmt.Errorf("load manifest schema: %w", err)
	}
	compiled, err := compiler.Compile("project.schema.json")
	if err != nil {
		return fmt.Errorf("compile manifest schema: %w", err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode manifest for validation: %w", err)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return fmt.Errorf("decode manifest for validation: %w", err)
	}
	if err := compiled.Validate(value); err != nil {
		return fmt.Errorf("JSON Schema validation: %w", err)
	}
	return nil
}

func manifestPaths(manifest domain.Manifest) []string {
	paths := []string{manifest.Repository.Root}
	if manifest.Runtime.Compose != nil {
		paths = append(paths, manifest.Runtime.Compose.Files...)
	}
	if manifest.Runtime.Process != nil {
		for _, process := range manifest.Runtime.Process.Processes {
			if process.WorkingDirectory != "" {
				paths = append(paths, process.WorkingDirectory)
			}
		}
	}
	for _, action := range manifest.Actions {
		if action.WorkingDirectory != "" {
			paths = append(paths, action.WorkingDirectory)
		}
	}
	return paths
}

func containedPath(root, candidate string) error {
	if candidate == "" {
		return nil
	}
	absolute := candidate
	if !filepath.IsAbs(candidate) {
		absolute = filepath.Join(root, candidate)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve trusted project root: %w", err)
	}
	canonicalCandidate, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return fmt.Errorf("resolve manifest path %q: %w", candidate, err)
	}
	relative, err := filepath.Rel(canonicalRoot, canonicalCandidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes the trusted project root", candidate)
	}
	return nil
}

func healthChecks(services []domain.Service) []domain.HealthCheck {
	var result []domain.HealthCheck
	for _, service := range services {
		result = append(result, service.HealthChecks...)
	}
	return result
}
