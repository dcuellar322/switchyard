package adapters

import (
	"strings"
	"testing"

	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

func TestRedactorSanitizesEveryLogFieldBeforeSinks(t *testing.T) {
	t.Parallel()
	redactor, err := NewRedactor([]string{`CUSTOM-[0-9]+`})
	if err != nil {
		t.Fatal(err)
	}
	redactor.AddSecret("known-secret-value")
	original := runtime.LogEntry{
		Message:    "Authorization: Bearer abc.def token=top-secret CUSTOM-123 known-secret-value postgres://user:pass@localhost/db",
		Attributes: map[string]string{"credential": "api_key=abc123", "safe": "visible"},
	}
	redacted := redactor.RedactLog(original)
	for _, secret := range []string{"abc.def", "top-secret", "CUSTOM-123", "known-secret-value", "pass", "abc123"} {
		encoded := redacted.Message + redacted.Attributes["credential"]
		if strings.Contains(encoded, secret) {
			t.Fatalf("redacted log contains %q: %#v", secret, redacted)
		}
	}
	if !redacted.Redacted || strings.Count(redacted.Message, redactionMarker) < 5 {
		t.Fatalf("redacted = %#v", redacted)
	}
	if original.Message == redacted.Message || original.Attributes["credential"] != "api_key=abc123" {
		t.Fatalf("source entry was mutated: %#v", original)
	}
}

func TestNewRedactorRejectsInvalidUserPattern(t *testing.T) {
	t.Parallel()
	if _, err := NewRedactor([]string{"["}); err == nil {
		t.Fatal("NewRedactor() error = nil")
	}
}
