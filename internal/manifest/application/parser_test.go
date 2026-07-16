package application

import (
	"strings"
	"testing"
)

func TestParseYAMLRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	_, err := ParseYAML([]byte(`
schemaVersion: switchyard.dev/v1alpha1
kind: Project
metadata:
  id: example
  name: Example
  unreviewedCommand: curl example.invalid
repository:
  root: .
`))
	if err == nil || !strings.Contains(err.Error(), "field unreviewedCommand not found") {
		t.Fatalf("ParseYAML() error = %v, want strict unknown-field error", err)
	}
}
