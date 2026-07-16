package adapters

import (
	"regexp"
	"strings"
	"sync"

	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

const redactionMarker = "[REDACTED]"

// Redactor applies one canonical secret policy before any log sink receives data.
type Redactor struct {
	mu       sync.RWMutex
	patterns []*regexp.Regexp
	secrets  []string
}

// NewRedactor compiles built-in credential formats and user-supplied regular expressions.
func NewRedactor(userPatterns []string) (*Redactor, error) {
	builtins := []string{
		`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`,
		`(?i)((?:password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key)\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`,
		`(?i)(://[^:/\s]+:)[^@/\s]+@`,
		`\bAKIA[0-9A-Z]{16}\b`,
	}
	patterns := make([]*regexp.Regexp, 0, len(builtins)+len(userPatterns))
	for _, expression := range append(builtins, userPatterns...) {
		compiled, err := regexp.Compile(expression)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, compiled)
	}
	return &Redactor{patterns: patterns}, nil
}

// AddSecret registers a resolved secret value without persisting it.
func (r *Redactor) AddSecret(value string) {
	if len(value) < 4 {
		return
	}
	r.mu.Lock()
	r.secrets = append(r.secrets, value)
	r.mu.Unlock()
}

// RedactLog returns an immutable copy with message and attribute values sanitized.
func (r *Redactor) RedactLog(entry runtime.LogEntry) runtime.LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry.Message, entry.Redacted = r.redact(entry.Message)
	attributes := make(map[string]string, len(entry.Attributes))
	for key, value := range entry.Attributes {
		redactedValue, changed := r.redact(value)
		attributes[key] = redactedValue
		entry.Redacted = entry.Redacted || changed
	}
	entry.Attributes = attributes
	return entry
}

func (r *Redactor) redact(value string) (string, bool) {
	result := value
	for _, secret := range r.secrets {
		result = strings.ReplaceAll(result, secret, redactionMarker)
	}
	for index, pattern := range r.patterns {
		switch index {
		case 0, 1, 2:
			result = pattern.ReplaceAllString(result, `${1}`+redactionMarker)
		default:
			result = pattern.ReplaceAllString(result, redactionMarker)
		}
	}
	return result, result != value
}
