package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"switchyard.dev/switchyard/internal/team/domain"
)

// RenderTemplate resolves declared variables and validates the complete manifest.
func (s *Service) RenderTemplate(ctx context.Context, bundleID string, values map[string]string) (json.RawMessage, error) {
	bundle, err := s.repository.GetBundle(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	if bundle.Kind != domain.KindProjectTemplate {
		return nil, fmt.Errorf("%w: bundle is not a project template", ErrInvalidBundle)
	}
	var template domain.ProjectTemplate
	if err := strictJSON(bundle.Payload, &template); err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(template.Variables))
	for _, variable := range template.Variables {
		value := values[variable.ID]
		if value == "" {
			value = variable.Default
		}
		if variable.Required && value == "" {
			return nil, fmt.Errorf("%w: template variable %s is required", ErrInvalidBundle, variable.ID)
		}
		resolved[variable.ID] = value
	}
	var document any
	if err := strictJSON(template.Manifest, &document); err != nil {
		return nil, fmt.Errorf("%w: template manifest: %v", ErrInvalidBundle, err)
	}
	document, err = substitute(document, resolved)
	if err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := s.manifests.ValidateManifestJSON(encoded); err != nil {
		return nil, fmt.Errorf("%w: rendered manifest: %v", ErrInvalidBundle, err)
	}
	return encoded, nil
}

func substitute(value any, variables map[string]string) (any, error) {
	switch current := value.(type) {
	case string:
		for id, replacement := range variables {
			current = strings.ReplaceAll(current, "{{"+id+"}}", replacement)
		}
		if strings.Contains(current, "{{") || strings.Contains(current, "}}") {
			return nil, fmt.Errorf("%w: unresolved template placeholder", ErrInvalidBundle)
		}
		return current, nil
	case []any:
		for index := range current {
			resolved, err := substitute(current[index], variables)
			if err != nil {
				return nil, err
			}
			current[index] = resolved
		}
		return current, nil
	case map[string]any:
		for key, item := range current {
			resolved, err := substitute(item, variables)
			if err != nil {
				return nil, err
			}
			current[key] = resolved
		}
		return current, nil
	default:
		return value, nil
	}
}
