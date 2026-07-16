package application

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	invopop "github.com/invopop/jsonschema"
	validator "github.com/santhosh-tekuri/jsonschema/v6"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
)

const providerOutputVersion = "switchyard.dev/ai-diagnosis/v1alpha1"

type providerHypothesis struct {
	ID          string   `json:"id" jsonschema:"required,pattern=^[a-zA-Z0-9._-]+$,maxLength=80"`
	Title       string   `json:"title" jsonschema:"required,maxLength=160"`
	Summary     string   `json:"summary" jsonschema:"required,maxLength=1000"`
	Severity    string   `json:"severity" jsonschema:"required,enum=info,enum=warning,enum=error"`
	Confidence  float64  `json:"confidence" jsonschema:"required,minimum=0,maximum=1"`
	EvidenceIDs []string `json:"evidenceIds" jsonschema:"required,minItems=1,maxItems=12"`
	ActionIDs   []string `json:"actionIds" jsonschema:"required,maxItems=3"`
}

type providerOutput struct {
	Version    string               `json:"version" jsonschema:"required,enum=switchyard.dev/ai-diagnosis/v1alpha1"`
	Hypotheses []providerHypothesis `json:"hypotheses" jsonschema:"required,maxItems=10"`
	Warnings   []string             `json:"warnings" jsonschema:"required,maxItems=10"`
}

func providerOutputSchema() (json.RawMessage, error) {
	reflector := invopop.Reflector{AllowAdditionalProperties: false, RequiredFromJSONSchemaTags: true}
	schema := reflector.Reflect(&providerOutput{})
	schema.ID = invopop.ID("https://switchyard.dev/schema/ai-diagnosis.v1alpha1.json")
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("encode AI diagnosis schema: %w", err)
	}
	return encoded, nil
}

func decodeProviderOutput(raw json.RawMessage) (providerOutput, error) {
	if len(raw) > 256<<10 {
		return providerOutput{}, errors.New("AI diagnosis response exceeds 256 KiB")
	}
	schema, err := providerOutputSchema()
	if err != nil {
		return providerOutput{}, err
	}
	compiler := validator.NewCompiler()
	var schemaDocument any
	if err := json.Unmarshal(schema, &schemaDocument); err != nil {
		return providerOutput{}, err
	}
	if err := compiler.AddResource("diagnosis.schema.json", schemaDocument); err != nil {
		return providerOutput{}, err
	}
	compiled, err := compiler.Compile("diagnosis.schema.json")
	if err != nil {
		return providerOutput{}, err
	}
	var document any
	if err := json.Unmarshal(raw, &document); err != nil {
		return providerOutput{}, fmt.Errorf("decode AI diagnosis: %w", err)
	}
	if err := compiled.Validate(document); err != nil {
		return providerOutput{}, fmt.Errorf("validate AI diagnosis: %w", err)
	}
	decoder := json.NewDecoder(io.LimitReader(bytes.NewReader(raw), int64(len(raw))+1))
	decoder.DisallowUnknownFields()
	var output providerOutput
	if err := decoder.Decode(&output); err != nil {
		return providerOutput{}, err
	}
	if output.Version != providerOutputVersion {
		return providerOutput{}, errors.New("AI diagnosis protocol version mismatch")
	}
	return output, nil
}

func providerBundle(bundle domain.Bundle) (json.RawMessage, error) {
	encoded, err := json.Marshal(struct {
		Version  string            `json:"version"`
		Project  string            `json:"projectId"`
		State    string            `json:"projectState"`
		Evidence []domain.Evidence `json:"evidence"`
		Actions  []domain.Action   `json:"approvedActions"`
	}{Version: bundle.Version, Project: bundle.ProjectID, State: bundle.ProjectState, Evidence: bundle.Evidence, Actions: bundle.Actions})
	if err != nil {
		return nil, err
	}
	if len(encoded) > maxBundleBytes {
		return nil, fmt.Errorf("provider evidence exceeds %d bytes", maxBundleBytes)
	}
	return encoded, nil
}

func validateProviderHypotheses(output providerOutput, bundle domain.Bundle) ([]domain.Hypothesis, []string) {
	evidence := make(map[string]bool, len(bundle.Evidence))
	for _, item := range bundle.Evidence {
		evidence[item.ID] = true
	}
	actions := make(map[string]domain.Action, len(bundle.Actions))
	for _, action := range bundle.Actions {
		actions[action.ID] = action
	}
	result := []domain.Hypothesis{}
	warnings := make([]string, 0, len(output.Warnings)+len(output.Hypotheses))
	for _, warning := range output.Warnings {
		warnings = append(warnings, normalizeProviderWarning(warning))
	}
	for _, proposed := range output.Hypotheses {
		if !allEvidenceExists(proposed.EvidenceIDs, evidence) {
			warnings = append(warnings, fmt.Sprintf("Provider hypothesis %q was rejected because it cited unknown evidence.", proposed.ID))
			continue
		}
		suggested, valid := providerActions(proposed.ActionIDs, actions)
		if !valid {
			warnings = append(warnings, fmt.Sprintf("Provider hypothesis %q was rejected because it requested an unavailable or unsafe action.", proposed.ID))
			continue
		}
		result = append(result, domain.Hypothesis{
			ID: "ai_" + proposed.ID, Code: "AI_HYPOTHESIS", Title: proposed.Title, Summary: proposed.Summary,
			Severity: proposed.Severity, Confidence: proposed.Confidence, Source: "ai", EvidenceIDs: slices.Clone(proposed.EvidenceIDs),
			SuggestedActions: suggested,
		})
	}
	return result, warnings
}

func allEvidenceExists(ids []string, evidence map[string]bool) bool {
	if len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if !evidence[id] {
			return false
		}
	}
	return true
}

func providerActions(ids []string, actions map[string]domain.Action) ([]domain.SuggestedAction, bool) {
	result := []domain.SuggestedAction{}
	seen := map[string]bool{}
	for _, id := range ids {
		action, ok := actions[id]
		if !ok || action.Risk == "destructive" || action.Risk == "networked" || action.Risk == "interactive" {
			return nil, false
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, domain.SuggestedAction{ActionID: id, Name: action.Name, Risk: action.Risk, Reason: "Existing approved project action cited by the optional provider."})
	}
	return result, true
}

func normalizeProviderWarning(value string) string {
	return boundedMessage(strings.ReplaceAll(value, "\n", " "), 500)
}
