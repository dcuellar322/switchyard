package application

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

const deterministicConfidenceFloor = 0.8

type mergeResult struct {
	Candidate  manifestDomain.Manifest
	Confidence map[string]float64
	Fields     []FieldReview
	Conflicts  []Conflict
	Warnings   []string
	DryRun     DryRun
}

type evidenceSupport struct {
	items           map[string]discoveryDomain.Evidence
	files           map[string]bool
	composeServices map[string]bool
	ports           map[string]bool
	commands        map[string]bool
}

func mergeProposal(root string, baseline discoveryDomain.Proposal, output ProposalOutput, validator CandidateValidator) (mergeResult, error) {
	support, claims, err := verifyClaims(baseline.Evidence, output.Claims)
	if err != nil {
		return mergeResult{}, err
	}
	result := mergeResult{
		Candidate: baseline.Candidate, Confidence: cloneConfidence(baseline.ConfidenceByField),
		Fields: []FieldReview{}, Conflicts: []Conflict{}, Warnings: append([]string(nil), output.Warnings...),
	}
	mergeScalar(&result, "/metadata/name", baseline.Candidate.Metadata.Name, output.Candidate.Metadata.Name, claims, support,
		func(value string) { result.Candidate.Metadata.Name = value })
	mergeScalar(&result, "/metadata/description", baseline.Candidate.Metadata.Description, output.Candidate.Metadata.Description, claims, support,
		func(value string) { result.Candidate.Metadata.Description = value })
	if !slices.Equal(baseline.Candidate.Metadata.Tags, output.Candidate.Metadata.Tags) && hasClaim(claims, "/metadata/tags") {
		result.Candidate.Metadata.Tags = uniqueSorted(append(append([]string(nil), baseline.Candidate.Metadata.Tags...), output.Candidate.Metadata.Tags...))
		result.addField("/metadata/tags", "ai", confidenceFor(claims["/metadata/tags"], support), claims["/metadata/tags"], nil)
	}
	mergeScalar(&result, "/repository/defaultBranch", baseline.Candidate.Repository.DefaultBranch, output.Candidate.Repository.DefaultBranch, claims, support,
		func(value string) { result.Candidate.Repository.DefaultBranch = value })
	if output.Candidate.Metadata.ID != baseline.Candidate.Metadata.ID {
		result.addConflict("/metadata/id", baseline.Candidate.Metadata.ID, output.Candidate.Metadata.ID)
	}
	if output.Candidate.Repository.Root != baseline.Candidate.Repository.Root {
		result.addConflict("/repository/root", baseline.Candidate.Repository.Root, output.Candidate.Repository.Root)
	}

	if !equalJSON(baseline.Candidate.Runtime, output.Candidate.Runtime) {
		claim := claims["/runtime"]
		if !hasClaim(claims, "/runtime") {
			result.reject("/runtime", claim, "provider runtime change has no verified evidence claim")
		} else if warning := safeRuntime(output.Candidate.Runtime, support); warning != "" {
			result.reject("/runtime", claim, warning)
		} else if confidenceAt(result.Confidence, "/runtime") >= deterministicConfidenceFloor && baseline.Candidate.Runtime.Driver != "" {
			result.addConflict("/runtime", baseline.Candidate.Runtime, output.Candidate.Runtime)
		} else {
			result.Candidate.Runtime = output.Candidate.Runtime
			result.Confidence["/runtime"] = confidenceFor(claim, support)
			result.addField("/runtime", "ai", result.Confidence["/runtime"], claim, nil)
		}
	}

	result.Candidate.Services = mergeServices(&result, baseline.Candidate.Services, output.Candidate.Services, claims["/services"], support)
	result.Candidate.Ports = mergePorts(&result, baseline.Candidate.Ports, output.Candidate.Ports, claims["/ports"], support)
	result.Candidate.Actions = mergeActions(&result, baseline.Candidate.Actions, output.Candidate.Actions, claims["/actions"], support)
	result.Candidate.Endpoints = mergeEndpoints(&result, baseline.Candidate.Endpoints, output.Candidate.Endpoints, claims["/endpoints"])
	// Lifecycle and resource policy stay deterministic in this phase; providers cannot create mutation semantics.
	if !equalJSON(baseline.Candidate.Lifecycle, output.Candidate.Lifecycle) {
		result.addConflict("/lifecycle", baseline.Candidate.Lifecycle, output.Candidate.Lifecycle)
	}
	if !equalJSON(baseline.Candidate.ResourcePolicy, output.Candidate.ResourcePolicy) {
		result.addConflict("/resourcePolicy", baseline.Candidate.ResourcePolicy, output.Candidate.ResourcePolicy)
	}

	validation := validator.Validate(root, result.Candidate)
	evidenceBacked := !hasRejected(result.Fields)
	result.DryRun = DryRun{
		Valid: validation.Valid && evidenceBacked, SchemaValid: validation.Valid, EvidenceBacked: evidenceBacked,
		RepositorySafe: validation.Valid, Errors: append([]string(nil), validation.Errors...),
		Warnings: append(append([]string(nil), validation.Warnings...), result.Warnings...),
	}
	return result, nil
}

func verifyClaims(evidence []discoveryDomain.Evidence, claims []FieldClaim) (evidenceSupport, map[string]FieldClaim, error) {
	support := evidenceSupport{items: map[string]discoveryDomain.Evidence{}, files: map[string]bool{}, composeServices: map[string]bool{}, ports: map[string]bool{}, commands: map[string]bool{}}
	for _, item := range evidence {
		support.items[item.ID] = item
		support.files[item.SourcePath] = true
		var data map[string]any
		_ = json.Unmarshal(item.Data, &data)
		if item.Kind == "compose.service" {
			support.composeServices[text(data["service"])] = true
		}
		if item.Kind == "compose.port" {
			support.ports[portKey(integer(data["host"]), integer(data["target"]), text(data["protocol"]), text(data["service"]))] = true
		}
		for _, key := range []string{"command", "testCommand"} {
			if command := stringsFrom(data[key]); len(command) > 0 {
				support.commands[commandKey(command)] = true
			}
		}
	}
	result := make(map[string]FieldClaim, len(claims))
	for _, claim := range claims {
		if !allowedClaimPath(claim.Path) {
			return evidenceSupport{}, nil, fmt.Errorf("%w: unsupported field claim %q", ErrProviderOutput, claim.Path)
		}
		if _, exists := result[claim.Path]; exists {
			return evidenceSupport{}, nil, fmt.Errorf("%w: duplicate field claim %q", ErrProviderOutput, claim.Path)
		}
		for _, id := range claim.EvidenceIDs {
			if _, exists := support.items[id]; !exists {
				return evidenceSupport{}, nil, fmt.Errorf("%w: claim %q references unknown evidence %q", ErrProviderOutput, claim.Path, id)
			}
		}
		result[claim.Path] = claim
	}
	return support, result, nil
}

func allowedClaimPath(path string) bool {
	return slices.Contains([]string{
		"/metadata/name", "/metadata/description", "/metadata/tags", "/repository/defaultBranch",
		"/runtime", "/services", "/ports", "/endpoints", "/actions",
	}, path)
}

func safeRuntime(runtime manifestDomain.Runtime, support evidenceSupport) string {
	switch runtime.Driver {
	case "compose":
		if runtime.Compose == nil || runtime.Process != nil || len(runtime.Compose.Files) == 0 {
			return "provider proposed an incomplete Compose runtime"
		}
		for _, file := range runtime.Compose.Files {
			if !support.files[file] {
				return fmt.Sprintf("provider proposed unobserved Compose file %q", file)
			}
		}
	case "process":
		if runtime.Process == nil || runtime.Compose != nil || len(runtime.Process.Processes) == 0 {
			return "provider proposed an incomplete process runtime"
		}
		if len(runtime.Process.Environment) > 0 || len(runtime.Process.Secrets) > 0 {
			return "provider-proposed process environment or secret requests are not accepted"
		}
		for _, process := range runtime.Process.Processes {
			if process.Shell || (process.WorkingDirectory != "" && process.WorkingDirectory != ".") || len(process.Environment) > 0 || len(process.Secrets) > 0 {
				return fmt.Sprintf("provider process %q requests unsupported shell, path, environment, or secret access", process.ID)
			}
			if !support.commands[commandKey(process.Command)] {
				return fmt.Sprintf("provider process %q command is not backed by deterministic evidence", process.ID)
			}
		}
	default:
		return fmt.Sprintf("provider proposed unsupported runtime driver %q", runtime.Driver)
	}
	return ""
}

func mergeServices(result *mergeResult, baseline, proposed []manifestDomain.Service, claim FieldClaim, support evidenceSupport) []manifestDomain.Service {
	values := append([]manifestDomain.Service(nil), baseline...)
	known := make(map[string]manifestDomain.Service, len(baseline))
	for _, item := range baseline {
		known[item.ID] = item
	}
	for _, item := range proposed {
		if existing, ok := known[item.ID]; ok {
			if !equalJSON(existing, item) {
				result.addConflict("/services/"+item.ID, existing, item)
			}
			continue
		}
		if len(claim.EvidenceIDs) == 0 {
			result.reject("/services/"+item.ID, claim, "provider service has no verified evidence claim")
			continue
		}
		if len(item.HealthChecks) > 0 || (item.Source.ComposeService != "" && !support.composeServices[item.Source.ComposeService]) {
			result.reject("/services/"+item.ID, claim, "provider service or health check is not backed by deterministic evidence")
			continue
		}
		if item.Source.Process == "" && item.Source.ComposeService == "" {
			result.reject("/services/"+item.ID, claim, "provider service has no supported runtime source")
			continue
		}
		values = append(values, item)
		result.addField("/services/"+item.ID, "ai", confidenceFor(claim, support), claim, nil)
	}
	slices.SortFunc(values, func(left, right manifestDomain.Service) int { return strings.Compare(left.ID, right.ID) })
	return values
}

func mergePorts(result *mergeResult, baseline, proposed []manifestDomain.Port, claim FieldClaim, support evidenceSupport) []manifestDomain.Port {
	values := append([]manifestDomain.Port(nil), baseline...)
	known := make(map[string]manifestDomain.Port, len(baseline))
	for _, item := range baseline {
		known[item.ID] = item
	}
	for _, item := range proposed {
		if existing, ok := known[item.ID]; ok {
			if !equalJSON(existing, item) {
				result.addConflict("/ports/"+item.ID, existing, item)
			}
			continue
		}
		if len(claim.EvidenceIDs) == 0 || !support.ports[portKey(item.Host, item.Target, item.Protocol, item.Service)] {
			result.reject("/ports/"+item.ID, claim, "provider port is not backed by deterministic evidence")
			continue
		}
		values = append(values, item)
		result.addField("/ports/"+item.ID, "ai", confidenceFor(claim, support), claim, nil)
	}
	slices.SortFunc(values, func(left, right manifestDomain.Port) int { return strings.Compare(left.ID, right.ID) })
	return values
}

func mergeActions(result *mergeResult, baseline, proposed []manifestDomain.Action, claim FieldClaim, support evidenceSupport) []manifestDomain.Action {
	values := append([]manifestDomain.Action(nil), baseline...)
	known := make(map[string]manifestDomain.Action, len(baseline))
	for _, item := range baseline {
		known[item.ID] = item
	}
	for _, item := range proposed {
		if existing, ok := known[item.ID]; ok {
			if !equalJSON(existing, item) {
				result.addConflict("/actions/"+item.ID, existing, item)
			}
			continue
		}
		if len(claim.EvidenceIDs) == 0 || item.Type != "command" || item.Shell || len(item.Environment) > 0 ||
			(item.WorkingDirectory != "" && item.WorkingDirectory != ".") || !support.commands[commandKey(item.Command)] {
			result.reject("/actions/"+item.ID, claim, "provider action, command, path, environment, or secret request is not backed by deterministic evidence")
			continue
		}
		values = append(values, item)
		result.addField("/actions/"+item.ID, "ai", confidenceFor(claim, support), claim, nil)
	}
	slices.SortFunc(values, func(left, right manifestDomain.Action) int { return strings.Compare(left.ID, right.ID) })
	return values
}

func mergeEndpoints(result *mergeResult, baseline, proposed []manifestDomain.Endpoint, claim FieldClaim) []manifestDomain.Endpoint {
	values := append([]manifestDomain.Endpoint(nil), baseline...)
	known := make(map[string]manifestDomain.Endpoint, len(baseline))
	ports := make(map[string]bool, len(result.Candidate.Ports))
	for _, port := range result.Candidate.Ports {
		ports[port.ID] = true
	}
	for _, item := range baseline {
		known[item.ID] = item
	}
	for _, item := range proposed {
		if existing, ok := known[item.ID]; ok {
			if !equalJSON(existing, item) {
				result.addConflict("/endpoints/"+item.ID, existing, item)
			}
			continue
		}
		portID := endpointPortID(item.URL)
		if len(claim.EvidenceIDs) == 0 || !ports[portID] {
			result.reject("/endpoints/"+item.ID, claim, "provider endpoint is not backed by a verified proposed port")
			continue
		}
		values = append(values, item)
	}
	return values
}

func mergeScalar(result *mergeResult, path, baseline, proposed string, claims map[string]FieldClaim, support evidenceSupport, accept func(string)) {
	if proposed == "" || proposed == baseline {
		return
	}
	claim, ok := claims[path]
	if !ok {
		result.reject(path, claim, "provider field change has no verified evidence claim")
		return
	}
	if confidenceAt(result.Confidence, path) >= deterministicConfidenceFloor && baseline != "" {
		result.addConflict(path, baseline, proposed)
		return
	}
	accept(proposed)
	confidence := confidenceFor(claim, support)
	result.Confidence[path] = confidence
	result.addField(path, "ai", confidence, claim, nil)
}

func (r *mergeResult) reject(path string, claim FieldClaim, warning string) {
	r.Warnings = append(r.Warnings, warning)
	r.addField(path, "rejected", confidenceFor(claim, evidenceSupport{items: map[string]discoveryDomain.Evidence{}}), claim, []string{warning})
}

func (r *mergeResult) addConflict(path string, deterministic, proposed any) {
	left, _ := json.Marshal(deterministic)
	right, _ := json.Marshal(proposed)
	r.Conflicts = append(r.Conflicts, Conflict{Path: path, DeterministicValue: left, ProposedValue: right, Resolution: "kept_deterministic"})
	r.Fields = append(r.Fields, FieldReview{Path: path, Source: "deterministic", Confidence: confidenceAt(r.Confidence, path), EvidenceIDs: []string{}, Rationale: "High-confidence deterministic evidence retained.", Warnings: []string{"Provider suggestion conflicted with deterministic evidence."}})
}

func (r *mergeResult) addField(path, source string, confidence float64, claim FieldClaim, warnings []string) {
	r.Fields = append(r.Fields, FieldReview{Path: path, Source: source, Confidence: confidence, EvidenceIDs: append([]string(nil), claim.EvidenceIDs...), Rationale: claim.Rationale, Warnings: nonNil(warnings)})
}

func confidenceFor(claim FieldClaim, support evidenceSupport) float64 {
	confidence := 0.0
	for _, id := range claim.EvidenceIDs {
		if item, ok := support.items[id]; ok && item.Confidence > confidence {
			confidence = item.Confidence
		}
	}
	if confidence > 0.79 {
		return 0.79
	}
	return confidence
}

func confidenceAt(values map[string]float64, path string) float64 {
	if value, ok := values[path]; ok {
		return value
	}
	return values["/"]
}

func hasClaim(values map[string]FieldClaim, path string) bool { _, ok := values[path]; return ok }
func hasRejected(values []FieldReview) bool {
	for _, value := range values {
		if value.Source == "rejected" {
			return true
		}
	}
	return false
}
func equalJSON(left, right any) bool {
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return string(a) == string(b)
}
func commandKey(command []string) string { return strings.Join(command, "\x00") }
func portKey(host, target int, protocol, service string) string {
	return fmt.Sprintf("%d/%d/%s/%s", host, target, protocol, service)
}
func text(value any) string { result, _ := value.(string); return result }
func integer(value any) int {
	if number, ok := value.(float64); ok {
		return int(number)
	}
	return 0
}
func stringsFrom(value any) []string {
	raw, _ := value.([]any)
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}
	return result
}
func uniqueSorted(values []string) []string {
	result := []string{}
	for _, value := range values {
		if value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	slices.Sort(result)
	return result
}
func endpointPortID(value string) string {
	start := strings.Index(value, "${ports.")
	if start < 0 {
		return ""
	}
	rest := value[start+8:]
	end := strings.Index(rest, "}")
	if end < 0 {
		return ""
	}
	return rest[:end]
}
func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
