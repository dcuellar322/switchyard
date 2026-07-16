package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

const maxExcerptBytes = 4 << 10

// BundlePreview is the user-visible consent receipt for an exact provider payload.
type BundlePreview struct {
	Bundle  EvidenceBundle  `json:"bundle"`
	Encoded json.RawMessage `json:"encoded"`
	SHA256  string          `json:"sha256"`
	Limits  Limits          `json:"limits"`
}

func buildBundle(
	ctx context.Context,
	catalog ProposalCatalog,
	reader EvidenceReader,
	redactor TextRedactor,
	proposalID string,
	limits Limits,
) (BundlePreview, error) {
	proposal, err := catalog.GetProposal(ctx, proposalID)
	if err != nil {
		return BundlePreview{}, err
	}
	project, err := catalog.GetProject(ctx, proposal.ProjectID)
	if err != nil {
		return BundlePreview{}, err
	}
	redactions := 0
	bundle := EvidenceBundle{
		Version: BundleVersion, ProjectID: proposal.ProjectID, ProposalID: proposal.ID,
		Candidate:         sanitizeManifest(proposal.Candidate, redactor, &redactions),
		ConfidenceByField: cloneConfidence(proposal.ConfidenceByField),
		Unresolved:        append([]string(nil), proposal.Unresolved...), Evidence: []EvidenceItem{},
	}
	bundle, redactions, err = appendEvidence(ctx, bundle, orderedEvidence(proposal.Evidence), project.PrimaryLocation, reader, redactor, redactions, limits.EvidenceBytes)
	if err != nil {
		return BundlePreview{}, err
	}
	bundle.RedactionCount = redactions
	encoded, err := stabilizedEncoding(&bundle)
	if err != nil {
		return BundlePreview{}, err
	}
	if int64(len(encoded)) > limits.EvidenceBytes {
		return BundlePreview{}, fmt.Errorf("evidence receipt exceeds %d-byte budget", limits.EvidenceBytes)
	}
	digest := sha256.Sum256(encoded)
	return BundlePreview{Bundle: bundle, Encoded: encoded, SHA256: hex.EncodeToString(digest[:]), Limits: limits}, nil
}

func orderedEvidence(values []discoveryDomain.Evidence) []discoveryEvidence {
	items := append([]discoveryEvidence(nil), wrapEvidence(values)...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SourcePath != items[j].SourcePath {
			return items[i].SourcePath < items[j].SourcePath
		}
		if items[i].Location.StartLine != items[j].Location.StartLine {
			return items[i].Location.StartLine < items[j].Location.StartLine
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func appendEvidence(
	ctx context.Context,
	bundle EvidenceBundle,
	items []discoveryEvidence,
	root string,
	reader EvidenceReader,
	redactor TextRedactor,
	redactions int,
	budget int64,
) (EvidenceBundle, int, error) {
	for _, source := range items {
		item, count, err := buildEvidenceItem(ctx, source, root, reader, redactor)
		if err != nil {
			return EvidenceBundle{}, redactions, err
		}
		redactions += count
		candidate, fits, err := appendEvidenceWithinBudget(bundle, item, redactions, budget)
		if err != nil {
			return EvidenceBundle{}, redactions, err
		}
		if !fits {
			bundle.Truncated = true
			break
		}
		bundle = candidate
	}
	return bundle, redactions, nil
}

func buildEvidenceItem(ctx context.Context, source discoveryEvidence, root string, reader EvidenceReader, redactor TextRedactor) (EvidenceItem, int, error) {
	data, changed, err := sanitizeJSON(source.Data, redactor)
	if err != nil {
		return EvidenceItem{}, 0, fmt.Errorf("sanitize evidence %q: %w", source.ID, err)
	}
	redactions := 0
	if changed {
		redactions++
	}
	item := EvidenceItem{
		ID: source.ID, Kind: source.Kind, SourcePath: filepath.ToSlash(source.SourcePath),
		Location: source.Location, Confidence: source.Confidence, Data: data,
		Warnings: append([]string(nil), source.Warnings...),
	}
	if reader == nil || source.Kind == "switchyard.manifest" || sensitivePath(source.SourcePath) {
		return item, redactions, nil
	}
	excerpt, truncated, readErr := reader.ReadExcerpt(ctx, root, source.SourcePath, source.Location, maxExcerptBytes)
	if readErr == nil {
		item.Excerpt, changed = redactor.RedactText(excerpt)
		item.Truncated = truncated
		if changed {
			redactions++
		}
	}
	return item, redactions, nil
}

func appendEvidenceWithinBudget(bundle EvidenceBundle, item EvidenceItem, redactions int, budget int64) (EvidenceBundle, bool, error) {
	candidate := bundle
	candidate.Evidence = append(append([]EvidenceItem(nil), bundle.Evidence...), item)
	candidate.RedactionCount = redactions
	encoded, err := json.Marshal(candidate)
	if err != nil {
		return EvidenceBundle{}, false, err
	}
	if int64(len(encoded)) <= budget {
		return candidate, true, nil
	}
	item.Excerpt, item.Truncated = "", true
	candidate.Evidence[len(candidate.Evidence)-1] = item
	encoded, err = json.Marshal(candidate)
	return candidate, err == nil && int64(len(encoded)) <= budget, err
}

func stabilizedEncoding(bundle *EvidenceBundle) ([]byte, error) {
	encoded, err := encodeBundle(*bundle)
	for err == nil && bundle.EncodedBytes != len(encoded) {
		bundle.EncodedBytes = len(encoded)
		encoded, err = encodeBundle(*bundle)
	}
	return encoded, err
}

func encodeBundle(bundle EvidenceBundle) ([]byte, error) {
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("encode evidence bundle: %w", err)
	}
	return encoded, nil
}

// discoveryEvidence is a local alias that keeps bundle construction readable.
type discoveryEvidence struct {
	ID         string
	Kind       string
	SourcePath string
	Location   discoveryDomain.SourceRange
	Confidence float64
	Data       json.RawMessage
	Warnings   []string
}

func wrapEvidence(values []discoveryDomain.Evidence) []discoveryEvidence {
	result := make([]discoveryEvidence, 0, len(values))
	for _, value := range values {
		result = append(result, discoveryEvidence{
			ID: value.ID, Kind: value.Kind, SourcePath: value.SourcePath, Location: value.Location,
			Confidence: value.Confidence, Data: value.Data, Warnings: value.Warnings,
		})
	}
	return result
}

func sanitizeJSON(source json.RawMessage, redactor TextRedactor) (json.RawMessage, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false, err
	}
	changed := sanitizeValue(value, "", redactor)
	encoded, err := json.Marshal(value)
	return encoded, changed, err
}

func sanitizeValue(value any, parent string, redactor TextRedactor) bool {
	changed := false
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			lower := strings.ToLower(key)
			if parent == "environment" || parent == "secrets" || isSecretKey(lower) {
				value[key] = "[REDACTED]"
				changed = true
				continue
			}
			if text, ok := child.(string); ok {
				if sanitized, redacted := redactor.RedactText(text); redacted {
					value[key] = sanitized
					changed = true
				}
				continue
			}
			if sanitizeValue(child, lower, redactor) {
				changed = true
			}
		}
	case []any:
		for index, child := range value {
			if text, ok := child.(string); ok {
				if sanitized, redacted := redactor.RedactText(text); redacted {
					value[index] = sanitized
					changed = true
				}
				continue
			}
			if sanitizeValue(child, parent, redactor) {
				changed = true
			}
		}
	}
	return changed
}

func sanitizeManifest(source manifestDomain.Manifest, redactor TextRedactor, redactions *int) manifestDomain.Manifest {
	encoded, _ := json.Marshal(source)
	var result manifestDomain.Manifest
	_ = json.Unmarshal(encoded, &result)
	if result.Runtime.Process != nil {
		redactEnvironment(result.Runtime.Process.Environment, redactions)
		redactSecretRefs(result.Runtime.Process.Secrets, redactions)
		for index := range result.Runtime.Process.Processes {
			redactStrings(result.Runtime.Process.Processes[index].Command, redactor, redactions)
			redactString(&result.Runtime.Process.Processes[index].WorkingDirectory, redactor, redactions)
			redactEnvironment(result.Runtime.Process.Processes[index].Environment, redactions)
			redactSecretRefs(result.Runtime.Process.Processes[index].Secrets, redactions)
		}
	}
	for index := range result.Actions {
		redactString(&result.Actions[index].Name, redactor, redactions)
		redactStrings(result.Actions[index].Command, redactor, redactions)
		redactString(&result.Actions[index].Target, redactor, redactions)
		redactEnvironment(result.Actions[index].Environment, redactions)
	}
	redactString(&result.Metadata.Name, redactor, redactions)
	redactString(&result.Metadata.Description, redactor, redactions)
	redactStrings(result.Metadata.Tags, redactor, redactions)
	for index := range result.Services {
		redactString(&result.Services[index].DisplayName, redactor, redactions)
		for checkIndex := range result.Services[index].HealthChecks {
			check := &result.Services[index].HealthChecks[checkIndex]
			redactString(&check.URL, redactor, redactions)
			redactString(&check.Address, redactor, redactions)
			redactString(&check.ExpectedValue, redactor, redactions)
			redactStrings(check.Command, redactor, redactions)
		}
	}
	for index := range result.Endpoints {
		redactString(&result.Endpoints[index].Name, redactor, redactions)
		redactString(&result.Endpoints[index].URL, redactor, redactions)
	}
	return result
}

func redactStrings(values []string, redactor TextRedactor, redactions *int) {
	for index := range values {
		redactString(&values[index], redactor, redactions)
	}
}

func redactString(value *string, redactor TextRedactor, redactions *int) {
	redacted, changed := redactor.RedactText(*value)
	if changed {
		*value = redacted
		(*redactions)++
	}
}

func redactEnvironment(values map[string]string, redactions *int) {
	for key, value := range values {
		if value != "" {
			values[key] = "[REDACTED]"
			(*redactions)++
		}
	}
}

func redactSecretRefs(values map[string]manifestDomain.SecretRef, redactions *int) {
	for key, value := range values {
		if value.Key != "" || value.Account != "" {
			value.Key, value.Account = "[REDACTED]", ""
			values[key] = value
			(*redactions)++
		}
	}
}

func cloneConfidence(source map[string]float64) map[string]float64 {
	result := make(map[string]float64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func sensitivePath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return base == ".env" || strings.HasPrefix(base, ".env.") || strings.Contains(base, "secret") || strings.Contains(base, "credential")
}

func isSecretKey(key string) bool {
	for _, part := range []string{"password", "passwd", "secret", "token", "api_key", "apikey", "private_key", "access_key", "credential"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}
