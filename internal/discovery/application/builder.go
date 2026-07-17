// Package application selects safe roots, scans evidence, and builds proposals.
package application

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"switchyard.dev/switchyard/internal/discovery/domain"
	manifest "switchyard.dev/switchyard/internal/manifest/domain"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// BuildProposal combines evidence by documented deterministic rules.
func BuildProposal(root Root, projectID, proposalID string, items []domain.Evidence) domain.Proposal {
	if candidate, ok := explicitManifest(items); ok {
		return domain.Proposal{
			ID: proposalID, ProjectID: projectID, ScannerVersion: domain.ScannerVersion,
			SchemaVersion: manifest.SchemaVersion, Candidate: candidate, Evidence: items,
			ConfidenceByField: map[string]float64{"/": 1}, Unresolved: unresolvedFields(candidate), Status: domain.StatusProposed,
		}
	}
	name := filepath.Base(root.Path)
	slug := slugify(name)
	candidate := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Kind:          manifest.KindProject,
		Metadata:      manifest.Metadata{ID: slug, Name: name, Tags: []string{}},
		Repository:    manifest.Repository{Root: "."},
		Services:      []manifest.Service{},
		Ports:         []manifest.Port{},
		Endpoints:     []manifest.Endpoint{},
		Actions:       []manifest.Action{},
	}
	confidence := map[string]float64{"/metadata/id": .85, "/metadata/name": .6, "/repository/root": 1}
	tags := map[string]bool{}
	actions := map[string]manifest.Action{}
	services := map[string]manifest.Service{}
	ports := map[string]manifest.Port{}
	for _, item := range items {
		var data map[string]any
		if json.Unmarshal(item.Data, &data) != nil {
			continue
		}
		applyEvidence(&candidate, confidence, tags, actions, services, ports, item, data)
	}
	if candidate.Runtime.Driver == "compose" {
		tags["compose"] = true
	}
	candidate.Services = sortedServices(services)
	candidate.Ports = sortedPorts(ports)
	candidate.Actions = sortedActions(actions)
	for tag := range tags {
		candidate.Metadata.Tags = append(candidate.Metadata.Tags, tag)
	}
	sort.Strings(candidate.Metadata.Tags)
	for index, service := range candidate.Services {
		confidence["/services/"+strconv.Itoa(index)] = bestConfidence(items, "compose.service", service.Source.ComposeService)
	}
	primaryPortID := primaryEndpointPort(candidate.Ports)
	for index, port := range candidate.Ports {
		confidence["/ports/"+strconv.Itoa(index)] = bestConfidence(items, "compose.port", port.Service)
		candidate.Endpoints = append(candidate.Endpoints, manifest.Endpoint{
			ID: port.ID, Name: port.ID, URL: "http://127.0.0.1:${ports." + port.ID + "}", Primary: port.ID == primaryPortID,
		})
	}
	unresolved := unresolvedFields(candidate)
	return domain.Proposal{
		ID: proposalID, ProjectID: projectID, ScannerVersion: domain.ScannerVersion,
		SchemaVersion: manifest.SchemaVersion, Candidate: candidate, Evidence: items,
		ConfidenceByField: confidence, Unresolved: unresolved, Status: domain.StatusProposed,
	}
}

func primaryEndpointPort(ports []manifest.Port) string {
	if len(ports) == 0 {
		return ""
	}
	bestID, bestRank := ports[0].ID, frontendServiceRank(ports[0].Service)
	for _, port := range ports[1:] {
		if rank := frontendServiceRank(port.Service); rank > bestRank {
			bestID, bestRank = port.ID, rank
		}
	}
	return bestID
}

func frontendServiceRank(service string) int {
	tokens := strings.FieldsFunc(strings.ToLower(service), func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for rank, preferred := range []string{"site", "app", "client", "ui", "web", "frontend"} {
		if slices.Contains(tokens, preferred) {
			return rank + 1
		}
	}
	return 0
}

func explicitManifest(items []domain.Evidence) (manifest.Manifest, bool) {
	for _, item := range items {
		if item.Kind != "switchyard.manifest" {
			continue
		}
		var candidate manifest.Manifest
		if json.Unmarshal(item.Data, &candidate) == nil {
			return candidate, true
		}
	}
	return manifest.Manifest{}, false
}

func applyEvidence(
	candidate *manifest.Manifest,
	confidence map[string]float64,
	tags map[string]bool,
	actions map[string]manifest.Action,
	services map[string]manifest.Service,
	ports map[string]manifest.Port,
	item domain.Evidence,
	data map[string]any,
) {
	switch item.Kind {
	case "git.repository":
		candidate.Repository.DefaultBranch = textValue(data, "defaultBranch")
		confidence["/repository/defaultBranch"] = item.Confidence
	case "readme.title":
		if title := textValue(data, "title"); title != "" {
			candidate.Metadata.Name = title
			confidence["/metadata/name"] = item.Confidence
		}
	case "compose.project":
		file := textValue(data, "file")
		candidate.Runtime = manifest.Runtime{Driver: "compose", Compose: &manifest.ComposeConfig{Files: []string{file}, ProjectName: textValue(data, "projectName"), Profiles: stringSlice(data["profiles"])}}
		candidate.Lifecycle = composeLifecycle()
		confidence["/runtime"] = item.Confidence
	case "compose.service":
		id := slugify(textValue(data, "service"))
		if id != "" {
			services[id] = manifest.Service{ID: id, DisplayName: textValue(data, "service"), Source: manifest.ServiceSource{ComposeService: textValue(data, "service")}, Dependencies: []string{}, HealthChecks: []manifest.HealthCheck{}}
		}
	case "compose.port":
		service := slugify(textValue(data, "service"))
		host, target := intValue(data, "host"), intValue(data, "target")
		id := service
		if _, exists := ports[id]; exists {
			id += "-" + strconv.Itoa(target)
		}
		ports[id] = manifest.Port{ID: id, Service: service, Host: host, Target: target, Protocol: textValue(data, "protocol")}
	case "python.project":
		tags["python"] = true
		if textValue(data, "manager") == "uv" {
			tags["uv"] = true
		}
		addCommandAction(actions, "test-python", "Run Python tests", stringSlice(data["testCommand"]), item.Confidence)
	case "node.script":
		tags["node"] = true
		script := textValue(data, "script")
		addCommandAction(actions, "node-"+slugify(script), "Run npm "+script, stringSlice(data["command"]), item.Confidence)
	case "make.target", "just.target":
		target := textValue(data, "target")
		addCommandAction(actions, strings.ReplaceAll(item.Kind, ".", "-")+"-"+slugify(target), "Run "+target, stringSlice(data["command"]), item.Confidence)
	}
}

func unresolvedFields(candidate manifest.Manifest) []string {
	unresolved := []string{}
	if candidate.Runtime.Driver == "" {
		unresolved = append(unresolved, "/runtime/driver")
	}
	if len(candidate.Services) == 0 {
		unresolved = append(unresolved, "/services")
	}
	return unresolved
}

func composeLifecycle() manifest.Lifecycle {
	return manifest.Lifecycle{
		Start: &manifest.Invocation{Action: "compose.up", Options: map[string]any{"detached": true}},
		Stop:  &manifest.Invocation{Action: "compose.stop"}, Restart: &manifest.Invocation{Action: "compose.restart"},
		Rebuild:  &manifest.Invocation{Action: "compose.up", Options: map[string]any{"build": true, "detached": true, "recreate": true}},
		Teardown: &manifest.Invocation{Action: "compose.down", Risk: "destructive"},
	}
}

func addCommandAction(values map[string]manifest.Action, id, name string, command []string, _ float64) {
	if len(command) == 0 {
		return
	}
	values[id] = manifest.Action{ID: id, Name: name, Type: "command", Command: command, WorkingDirectory: ".", CaptureOutput: true}
}

func slugify(value string) string {
	value = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if len(value) > 63 {
		value = strings.TrimRight(value[:63], "-")
	}
	if value == "" {
		return "project"
	}
	return value
}

func textValue(data map[string]any, key string) string { value, _ := data[key].(string); return value }
func intValue(data map[string]any, key string) int {
	value, _ := data[key].(float64)
	return int(value)
}
func stringSlice(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func sortedServices(values map[string]manifest.Service) []manifest.Service {
	keys := sortedKeys(values)
	result := make([]manifest.Service, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}
func sortedPorts(values map[string]manifest.Port) []manifest.Port {
	keys := sortedKeys(values)
	result := make([]manifest.Port, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}
func sortedActions(values map[string]manifest.Action) []manifest.Action {
	keys := sortedKeys(values)
	result := make([]manifest.Action, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}
func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func bestConfidence(items []domain.Evidence, kind, contains string) float64 {
	var result float64
	for _, item := range items {
		if item.Kind == kind && strings.Contains(string(item.Data), `"`+contains+`"`) && item.Confidence > result {
			result = item.Confidence
		}
	}
	return result
}
