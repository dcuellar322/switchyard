package identifier

import (
	"strings"
	"testing"
)

func TestNewUsesPrefixAndUniqueEntropy(t *testing.T) {
	t.Parallel()

	first, err := New("op")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	second, err := New("op")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !strings.HasPrefix(first, "op_") || first == second {
		t.Fatalf("identifiers = %q, %q", first, second)
	}
}
